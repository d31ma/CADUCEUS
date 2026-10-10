import { join } from "node:path";
import { rmSync } from "node:fs";
import { ServerEnvironment } from "../../server/platform/runtime/ServerEnvironment.js";
import { createServer } from "node:net";
import { createHmac } from "node:crypto";
import { startIdentity, identityRequirements } from "../support/identity.mjs";
const CADUCEUS_ROOT = join(import.meta.dir, "..", "..");
const INBOUND_WEBHOOK_SECRET = "harness-inbound-secret";

export { identityRequirements };
export const TEST_SERVER_HOOK_TIMEOUT_MS =
  (Number(process.env.TEST_SERVER_START_TIMEOUT_MS) || 300_000) + 5_000;

export async function startTestServer() {
  const port = await availableLoopbackPort();
  // The Go owner holds the mail root and the engine. The application server
  // reaches both through its bridge, exactly as it does in production, and
  // tokens come from real logins rather than from this file.
  const identity = await startIdentity();
  const testRoot = identity.fyloRoot;
  // Serve the real application with Tachyon. Its handler processes read their
  // settings from the project .env, exactly as the entry point hands them over.
  const envFile = join(CADUCEUS_ROOT, ".env");
  const existing = await Bun.file(envFile).text().catch(() => null);
  if (existing !== null && !existing.startsWith(ServerEnvironment.GENERATED_MARKER)) {
    throw new Error(`${envFile} was not written by CADUCEUS; move it aside before running the API suites`);
  }
  const settings = ServerEnvironment.dotenv({
    ...identity.env,
    INBOUND_WEBHOOK_SECRET,
    NODE_ENV: "test",
    CADUCEUS_ENABLE_TEST_ROUTES: "true",
    STREAM_MAX_MS: "750",
    STREAM_POLL_MS: "50",
  });
  await Bun.write(envFile, settings.text);
  const proc = Bun.spawn([process.env.TACHYON_BIN || "ty", "serve", "--no-watch", "--port", String(port)], {
    cwd: CADUCEUS_ROOT,
    env: { PATH: process.env.PATH, HOME: process.env.HOME },
    stdout: "ignore",
    stderr: "ignore"
  });
  const url = `http://127.0.0.1:${port}`;
  const startupTimeoutMs = Number(process.env.TEST_SERVER_START_TIMEOUT_MS) || 300_000;
  const deadline = Date.now() + startupTimeoutMs;
  let ready = false;
  while (Date.now() < deadline) {
    try {
      // Tachyon answers 401 for a @SignedIn route once the application is
      // built, so the first test cannot be served by the frontend fallback.
      const r = await fetch(`${url}/domains`, {
        signal: AbortSignal.timeout(500)
      });
      if (r.status === 401) {
        ready = true;
        break;
      }
    } catch {
      await Bun.sleep(100);
    }
  }
  const stop = () => {
    proc.kill();
    identity.stop();
    rmSync(envFile, { force: true });
  };
  if (!ready) {
    stop();
    throw new Error(`Test server did not become ready at ${url} within ${startupTimeoutMs}ms`);
  }
  const makeHeaders = (body, opts) => {
    const h = { "Content-Type": "application/json" };
    if (opts?.token)
      h["Authorization"] = `Bearer ${opts.token}`;
    if (opts?.secret)
      h["X-Webhook-Secret"] = opts.secret;
    if (opts?.secret)
      h["X-Caduceus-Signature"] = createHmac("sha256", opts.secret).update(JSON.stringify(body ?? {})).digest("hex");
    return h;
  };
  return {
    url,
    testRoot,
    stop,
    identity,
    // Provision one mailbox and sign in as it. `claims` keeps the shape the
    // suites already pass; only `email` and `role` mean anything now, because
    // a principal's domain is its address's and its authority is a grant.
    token: async (claims = {}) => {
      const seeded = await identity.mint(
        claims.email ?? "alice@example.com",
        claims.role ?? "viewer",
      );
      return seeded.token;
    },
    mint: (address, role) => identity.mint(address, role),
    get: (path, opts) => fetch(`${url}${path}`, { headers: makeHeaders(undefined, opts) }),
    post: (path, body, opts) => fetch(`${url}${path}`, {
      method: "POST",
      headers: makeHeaders(body, { ...opts, secret: opts?.secret ?? (
        path === "/inbound/webhook" ? INBOUND_WEBHOOK_SECRET :
        undefined
      ) }),
      body: JSON.stringify(body)
    }),
    put: (path, body, opts) => fetch(`${url}${path}`, { method: "PUT", headers: makeHeaders(body, opts), body: JSON.stringify(body) }),
    delete: (path, opts) => fetch(`${url}${path}`, {
      method: "DELETE",
      headers: makeHeaders(opts?.body, opts),
      body: opts?.body ? JSON.stringify(opts.body) : undefined
    })
  };
}

async function availableLoopbackPort() {
  return await new Promise((resolve, reject) => {
    const reservation = createServer();
    reservation.unref();
    reservation.once("error", reject);
    reservation.listen(0, "127.0.0.1", () => {
      const address = reservation.address();
      if (!address || typeof address === "string") {
        reservation.close();
        reject(new Error("Unable to reserve a loopback test port"));
        return;
      }
      reservation.close(error => error ? reject(error) : resolve(address.port));
    });
  });
}
