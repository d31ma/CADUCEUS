import { describe, it, expect, beforeAll, afterAll } from "bun:test";
import { startTestServer, TEST_SERVER_HOOK_TIMEOUT_MS } from "./helpers.js";

/**
 * The authentication contract, over HTTP.
 *
 * The Go suite proves the boundary's behaviour against the engine directly.
 * This proves the same contract through the transport the browser actually
 * uses, so the client and the server are checked against one corpus rather
 * than two descriptions of it.
 */

let s;
beforeAll(async () => {
  s = await startTestServer();
}, TEST_SERVER_HOOK_TIMEOUT_MS);
afterAll(() => s?.stop());

/** Seed one mailbox and return its bootstrap credentials. */
async function seed(email = "alice@example.com", role = "admin") {
  const r = await s.post("/test/seed/user", { email, domains: [email.split("@")[1]], role });
  expect(r.status).toBe(200);
  return await r.json();
}

describe("POST /auth/login/begin", () => {
  it("requires an address", async () => {
    const r = await s.post("/auth/login/begin", {});
    expect(r.status).toBe(400);
  });

  it("opens a transaction and reports the required factors", async () => {
    await seed("begin@example.com");
    const r = await s.post("/auth/login/begin", { address: "begin@example.com" });
    expect(r.status).toBe(200);
    const state = await r.json();
    expect(state.transactionId).toBeTruthy();
    expect(state.requiredFactors).toContain("passkey");
    expect(state.satisfied).toBe(false);
  });

  it("does not reveal whether a mailbox exists", async () => {
    const known = await s.post("/auth/login/begin", { address: "begin@example.com" });
    const unknown = await s.post("/auth/login/begin", { address: "nobody@example.com" });
    expect(known.status).toBe(unknown.status);
    const [a, b] = [await known.json(), await unknown.json()];
    expect(Object.keys(a).sort()).toEqual(Object.keys(b).sort());
    expect(a.state).toBe(b.state);
  });
});

describe("the login ladder", () => {
  it("refuses to complete before the policy is satisfied", async () => {
    const seeded = await seed("ladder@example.com");
    const begun = await (await s.post("/auth/login/begin", { address: seeded.email })).json();

    const password = await s.post("/auth/login/password", {
      transactionId: begun.transactionId, password: seeded.password,
    });
    expect(password.status).toBe(200);
    expect((await password.json()).satisfied).toBe(false);

    const early = await s.post("/auth/login/complete", { transactionId: begun.transactionId });
    expect(early.status).toBe(403);

    // The rejected completion is not terminal: the same transaction finishes
    // once the second factor lands.
    const recovery = await s.post("/auth/login/recovery", {
      transactionId: begun.transactionId, code: seeded.recoveryCodes[0],
    });
    expect(recovery.status).toBe(200);
    expect((await recovery.json()).satisfied).toBe(true);

    const completed = await s.post("/auth/login/complete", { transactionId: begun.transactionId });
    expect(completed.status).toBe(200);
    const tokens = await completed.json();
    expect(tokens.accessToken).toBeTruthy();
    expect(tokens.refreshToken).toBeTruthy();
    expect(tokens.idToken).toBeTruthy();
    expect(tokens.address).toBe("ladder@example.com");
  });

  it("rejects a wrong password without advancing", async () => {
    const seeded = await seed("wrong@example.com");
    const begun = await (await s.post("/auth/login/begin", { address: seeded.email })).json();
    const r = await s.post("/auth/login/password", {
      transactionId: begun.transactionId, password: "not the password",
    });
    const state = await r.json();
    expect(state.satisfied).toBe(false);
  });

  it("refuses a recovery code offered before any primary factor", async () => {
    const seeded = await seed("first@example.com");
    const begun = await (await s.post("/auth/login/begin", { address: seeded.email })).json();
    const r = await s.post("/auth/login/recovery", {
      transactionId: begun.transactionId, code: seeded.recoveryCodes[0],
    });
    expect(r.status).toBeGreaterThanOrEqual(400);
  });

  it("spends a recovery code once", async () => {
    const seeded = await seed("once@example.com");
    const code = seeded.recoveryCodes[0];
    const first = await (await s.post("/auth/login/begin", { address: seeded.email })).json();
    await s.post("/auth/login/password", { transactionId: first.transactionId, password: seeded.password });
    expect((await (await s.post("/auth/login/recovery", {
      transactionId: first.transactionId, code,
    })).json()).satisfied).toBe(true);

    const second = await (await s.post("/auth/login/begin", { address: seeded.email })).json();
    await s.post("/auth/login/password", { transactionId: second.transactionId, password: seeded.password });
    const replayed = await s.post("/auth/login/recovery", {
      transactionId: second.transactionId, code,
    });
    const state = replayed.ok ? await replayed.json() : { satisfied: false };
    expect(state.satisfied).toBe(false);
  });
});

describe("GET /auth/session", () => {
  it("requires a token", async () => {
    const r = await s.get("/auth/session");
    expect(r.status).toBe(401);
  });

  it("resolves the caller", async () => {
    const token = await s.token({ email: "session@example.com", role: "admin" });
    const r = await s.get("/auth/session", { token });
    expect(r.status).toBe(200);
    const principal = await r.json();
    expect(principal.address).toBe("session@example.com");
    expect(principal.domain).toBe("example.com");
    expect(principal.principalId).toBeTruthy();
  });

  it("refuses a malformed, forged, or downgraded token", async () => {
    const token = await s.token({ email: "forge@example.com", role: "admin" });
    const [header, payload, signature] = token.split(".");
    for (const candidate of [
      "not-a-token",
      `${header}.${payload}`,
      `${header}.${payload}.`,
      `${header}.${payload}.${signature.split("").reverse().join("")}`,
      `${btoa('{"alg":"none","typ":"JWT"}').replace(/=+$/, "")}.${payload}.`,
    ]) {
      const r = await s.get("/auth/session", { token: candidate });
      expect(r.status).toBe(401);
    }
  });
});

describe("DELETE /auth/session", () => {
  it("ends the session, and the token stops working immediately", async () => {
    const token = await s.token({ email: "revoke@example.com", role: "admin" });
    expect((await s.get("/auth/session", { token })).status).toBe(200);

    const revoked = await s.delete("/auth/session", { token });
    expect(revoked.status).toBe(200);

    // Not at expiry — now.
    expect((await s.get("/auth/session", { token })).status).toBe(401);
    expect((await s.get("/domains", { token })).status).toBe(401);
  });
});

describe("POST /auth/token/refresh", () => {
  it("rotates the refresh token and refuses the consumed one", async () => {
    const seeded = await s.mint("refresh@example.com", "admin");
    const clientId = JSON.parse(
      atob(seeded.accessToken.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")),
    ).aud;

    const rotated = await s.post("/auth/token/refresh", {
      refreshToken: seeded.refreshToken, clientId,
    });
    expect(rotated.status).toBe(200);
    const next = await rotated.json();
    expect(next.refreshToken).not.toBe(seeded.refreshToken);

    const replayed = await s.post("/auth/token/refresh", {
      refreshToken: seeded.refreshToken, clientId,
    });
    expect(replayed.status).toBeGreaterThanOrEqual(400);
  });
});

describe("GET /auth/jwks", () => {
  it("publishes the public signing key and no private material", async () => {
    const r = await s.get("/auth/jwks");
    expect(r.status).toBe(200);
    const { keys } = await r.json();
    expect(keys.length).toBeGreaterThan(0);
    for (const key of keys) {
      expect(key.kty).toBe("EC");
      expect(key.alg).toBe("ES256");
      expect(key.d).toBeUndefined();
    }
  });
});

describe("credential management", () => {
  it("refuses a change without a satisfied step-up", async () => {
    const token = await s.token({ email: "credential@example.com", role: "admin" });
    const r = await s.post("/auth/credentials/recovery", {
      stepUpTransactionId: "atx_not_a_real_transaction",
    }, { token });
    expect(r.status).toBeGreaterThanOrEqual(400);
  });

  it("reissues recovery codes after a step-up", async () => {
    const seeded = await s.mint("stepup@example.com", "admin");
    const begun = await (await s.post("/auth/login/begin", { address: seeded.email })).json();
    await s.post("/auth/login/password", { transactionId: begun.transactionId, password: seeded.password });
    await s.post("/auth/login/recovery", {
      transactionId: begun.transactionId, code: seeded.recoveryCodes[0],
    });

    const r = await s.post("/auth/credentials/recovery", {
      stepUpTransactionId: begun.transactionId,
    }, { token: seeded.token });
    expect(r.status).toBe(200);
    expect((await r.json()).codes.length).toBeGreaterThan(0);
  });
});
