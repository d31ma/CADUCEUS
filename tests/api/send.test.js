import { describe, it, expect, beforeAll, afterAll } from "bun:test";
import { startTestServer, TEST_SERVER_HOOK_TIMEOUT_MS } from "./helpers.js";
let s;
let token;
beforeAll(async () => {
  s = await startTestServer();
  token = await s.token({ email: "alice@example.com", domains: ["example.com"], role: "admin" });
}, TEST_SERVER_HOOK_TIMEOUT_MS);
afterAll(() => s?.stop());
describe("POST /send", () => {
  it("returns 401 without token", async () => {
    const r = await s.post("/send", { to: ["bob@example.com"], subject: "Hi", text: "Hello" });
    expect(r.status).toBe(401);
  });
  it("returns 400 when to is missing", async () => {
    const r = await s.post("/send", { subject: "Hi", text: "Hello" }, { token });
    expect(r.status).toBe(400);
  });
  it("returns 400 when subject is missing", async () => {
    const r = await s.post("/send", { to: ["bob@example.com"], text: "Hello" }, { token });
    expect(r.status).toBe(400);
  });
  it("returns 400 when neither text nor html is provided", async () => {
    const r = await s.post("/send", { to: ["bob@example.com"], subject: "Hi" }, { token });
    expect(r.status).toBe(400);
  });
  it("rejects header injection in subject", async () => {
    const r = await s.post("/send", {
      to: ["bob@example.com"],
      subject: `Hello\r
Bcc: attacker@example.com`,
      text: "Test"
    }, { token });
    expect(r.status).toBe(400);
  });
  it("rejects oversized recipient lists", async () => {
    const recipients = Array.from({ length: 51 }, (_, i) => `user${i}@example.com`);
    const r = await s.post("/send", {
      to: recipients,
      subject: "Hello",
      text: "Test"
    }, { token });
    expect(r.status).toBe(400);
  });
  it("sends email (console adapter) and returns messageId", async () => {
    const r = await s.post("/send", {
      to: ["bob@example.com"],
      subject: "Hello",
      text: "Test message"
    }, { token });
    expect(r.status).toBe(200);
    const body = await r.json();
    expect(typeof body.messageId).toBe("string");
  });
  it("stores an untracked sent copy when tracking is omitted", async () => {
    const subject = `No tracking ${Date.now()}`;
    const r = await s.post("/send", {
      to: ["bob@example.com"],
      subject,
      text: "Test message"
    }, { token });
    expect(r.status).toBe(200);

    const sent = await s.get("/inbox?folder=sent", { token });
    expect(sent.status).toBe(200);
    const emails = await sent.json();
    const email = emails.find((item) => item.subject === subject);
    expect(email).toBeTruthy();
    expect(email.folder).toBe("sent");
    expect(email.trackingId).toBeUndefined();
    expect(email.openTracking).toBeUndefined();
    expect(email.linkTracking).toBeUndefined();
  });
  it("stores sent attachment metadata and content", async () => {
    const subject = `Attachment send ${Date.now()}`;
    const r = await s.post("/send", {
      to: ["bob@example.com"],
      subject,
      text: "See attached",
      attachments: [{
        filename: "note.txt",
        contentType: "text/plain",
        contentBase64: Buffer.from("sent attachment").toString("base64")
      }]
    }, { token });
    expect(r.status).toBe(200);
    const body = await r.json();

    const sent = await s.get(`/inbox/${body.messageId}`, { token });
    expect(sent.status).toBe(200);
    const email = await sent.json();
    expect(email.attachments).toHaveLength(1);
    expect(email.attachments[0].filename).toBe("note.txt");
    expect(email.attachments[0].storagePath).toBeUndefined();

    const attachment = await s.get(`/inbox/${body.messageId}/attachments/${email.attachments[0].id}`, { token });
    expect(attachment.status).toBe(200);
    const payload = await attachment.json();
    expect(Buffer.from(payload.contentBase64, "base64").toString("utf8")).toBe("sent attachment");
  });
  it("rejects invalid attachment payloads", async () => {
    const r = await s.post("/send", {
      to: ["bob@example.com"],
      subject: "Bad attachment",
      text: "Test",
      attachments: [{ filename: "bad.txt", contentType: "text/plain", contentBase64: "not base64!" }]
    }, { token });
    expect(r.status).toBe(400);
  });
  it("returns 422 when recipient is suppressed", async () => {
    await s.post("/suppressed", { address: "suppressed@example.com", reason: "bounce" }, { token });
    const r = await s.post("/send", {
      to: ["suppressed@example.com"],
      subject: "Hello",
      text: "Test"
    }, { token });
    expect(r.status).toBe(422);
    const body = await r.json();
    expect(body.blocked).toContain("suppressed@example.com");
  });
});
