import { describe, it, expect, beforeAll, afterAll } from "bun:test";
import { startTestServer, TEST_SERVER_HOOK_TIMEOUT_MS } from "./helpers.js";

let s;
let adminToken;
let otherToken;

beforeAll(async () => {
  s = await startTestServer();
  adminToken = await seedUser("spam-admin@example.com", ["example.com"]);
  otherToken = await seedUser("other-admin@other.com", ["other.com"]);
  await seedDomain(adminToken, "example.com");
  await seedDomain(otherToken, "other.com");
}, TEST_SERVER_HOOK_TIMEOUT_MS);

afterAll(() => s?.stop());

describe("PUT /inbox/:id/spam", () => {
  it("returns 401 without token", async () => {
    const id = await deliverClean("401");
    const r = await s.put(`/inbox/${id}/spam`, { spam: true });
    expect(r.status).toBe(401);
  });

  it("returns 403 for an email outside the caller's domains", async () => {
    const id = await deliverClean("forbidden");
    const r = await s.put(`/inbox/${id}/spam`, { spam: true }, { token: otherToken });
    expect(r.status).toBe(403);
  });

  it("reports an inbox email as spam", async () => {
    const id = await deliverClean("report");
    const r = await s.put(`/inbox/${id}/spam`, { spam: true }, { token: adminToken });
    expect(r.status).toBe(200);
    const updated = await r.json();
    expect(updated.id).toBe(id);
    expect(updated.folder).toBe("spam");
    expect(updated.spamReasons).toEqual(["user-reported"]);

    const spam = await s.get("/inbox?folder=spam", { token: adminToken });
    const messages = await spam.json();
    expect(messages.some((email) => email.id === id)).toBe(true);
  });

  it("marks spam as not spam and clears spam metadata", async () => {
    const id = await deliverSpam("not-spam");
    const spam = await s.get(`/inbox/${id}`, { token: adminToken });
    const spamEmail = await spam.json();
    expect(spamEmail.folder).toBe("spam");
    expect(Number(spamEmail.spamScore)).toBeGreaterThanOrEqual(5);

    const r = await s.put(`/inbox/${id}/spam`, { spam: false }, { token: adminToken });
    expect(r.status).toBe(200);
    const updated = await r.json();
    expect(updated.folder).toBe("inbox");
    expect(updated.spamScore).toBeUndefined();
    expect(updated.spamReasons).toBeUndefined();

    const fetched = await s.get(`/inbox/${id}`, { token: adminToken });
    const after = await fetched.json();
    expect(after.folder).toBe("inbox");
    expect(after.spamScore).toBeUndefined();
    expect(after.spamReasons).toBeUndefined();
  });

});

async function seedUser(email, domains) {
  const r = await s.post("/test/seed/user", {
    email,
    domains,
    role: "admin",
  });
  expect(r.status).toBe(200);
  const body = await r.json();
  return body.token;
}

async function seedDomain(token, domain) {
  const r = await s.post("/domains", {
    domain,
    inboundEnabled: true,
    routes: [{ id: `store-${domain}`, match: `*@${domain}`, action: { type: "store" }, enabled: true }],
  }, { token });
  expect(r.status).toBe(200);
}

async function deliverClean(label) {
  return deliver({
    recipient: "spam-admin@example.com",
    sender: "sender@other.com",
    subject: `Clean ${label} ${Date.now()}`,
    body: "Hello from a regular sender.",
  });
}

async function deliverSpam(label) {
  return deliver({
    recipient: "spam-admin@example.com",
    sender: "sender@other.com",
    subject: `Spam ${label} ${Date.now()}`,
    body: "The local receiver marked this message as spam.",
    spamVerdict: "FAIL",
  });
}

async function deliver(message) {
  const r = await s.post("/inbound/webhook", message);
  expect(r.status).toBe(200);
  const body = await r.json();
  expect(body.accepted).toBe(true);
  expect(typeof body.emailId).toBe("string");
  return body.emailId;
}
