import { join } from "node:path";
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
  // Run the decorated route classes through the same bounded Bun adapter used
  // by Playwright. Tachyon 26.33's isolated Yon supervisor cannot load the
  // legacy shared-service imports while those routes are migrating to Go.
  const serveCommand = ["bun", "scripts/e2e-api.mjs"];
  const proc = Bun.spawn(serveCommand, {
    cwd: CADUCEUS_ROOT,
    env: {
      ...identity.env,
      E2E_API_PORT: String(port),
      CADUCEUS_TEST_USE_EXISTING_IDENTITY: "true",
    },
    stdout: "ignore",
    stderr: "ignore"
  });
  const url = `http://127.0.0.1:${port}`;
  const startupTimeoutMs = Number(process.env.TEST_SERVER_START_TIMEOUT_MS) || 300_000;
  const deadline = Date.now() + startupTimeoutMs;
  let ready = false;
  while (Date.now() < deadline) {
    try {
      // Tachyon can answer its own health route before CADUCEUS routes finish
      // loading. Require a stable application response so the first test
      // cannot be served by the frontend fallback.
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
