import { describe, it, expect, beforeEach, afterEach } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { getApp, resetApp } from "@/app.js";

let testRoot;
let emailsRepo;
let domainsRepo;
const previousFyloRoot = process.env.FYLO_ROOT;

beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-spam-"));
  process.env.FYLO_ROOT = testRoot;
  await resetApp();
  const app = await getApp();
  emailsRepo = app.emails;
  domainsRepo = app.domains;
  await domainsRepo.put({
    domain: "example.com",
    inboundEnabled: true,
    routes: [{ id: "store", match: "*@example.com", action: { type: "store" }, enabled: true }],
  });
});

afterEach(async () => {
  await resetApp();
  if (previousFyloRoot === undefined) delete process.env.FYLO_ROOT;
  else process.env.FYLO_ROOT = previousFyloRoot;
  rmSync(testRoot, { recursive: true, force: true });
});

describe("inbound spam routing", () => {
  it("delivers clean mail to the inbox", async () => {
    const app = await getApp();
    const res = await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "bob@friend.com",
      subject: "Lunch tomorrow?",
      body: "Want to grab lunch next week?",
      messageId: "clean-1",
    });
    expect(res.body.accepted).toBe(true);
    expect(res.body.spam).toBeUndefined();
    const [, email] = await emailsRepo.findById("clean-1");
    expect(email.folder).toBe("inbox");
  });

  it("routes a receiver FAIL verdict to the spam folder", async () => {
    const app = await getApp();
    const res = await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "newsletter@store.com",
      subject: "Your receipt",
      body: "Thanks for your order",
      messageId: "spam-verdict",
      spamVerdict: "FAIL",
    });
    expect(res.body.spam).toBe(true);
    const [, email] = await emailsRepo.findById("spam-verdict");
    expect(email.folder).toBe("spam");
    expect(Number(email.spamScore)).toBeGreaterThanOrEqual(5);
  });

  it("routes mail carrying X-Spam headers to the spam folder", async () => {
    const app = await getApp();
    const mime = [
      "X-Spam-Flag: YES",
      "X-Spam-Status: Yes, score=8.1 required=5.0",
      "Content-Type: text/plain",
      "",
      "Buy now",
    ].join("\r\n");
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "x@y.com",
      subject: "hello",
      body: mime,
      messageId: "spam-header",
    });
    const [, email] = await emailsRepo.findById("spam-header");
    expect(email.folder).toBe("spam");
  });

  it("quarantines obvious heuristic junk and skips inbox rules", async () => {
    const app = await getApp();
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "win@lottery-prize.win",
      subject: "CONGRATULATIONS YOU HAVE WON!!!",
      body: "Claim now! Wire transfer your bitcoin to collect the jackpot.",
      messageId: "spam-heuristic",
    });
    const [, email] = await emailsRepo.findById("spam-heuristic");
    expect(email.folder).toBe("spam");
  });

  it("stores Reply-To metadata and warns when it differs from the visible sender", async () => {
    const app = await getApp();
    const mime = [
      "From: Billing <billing@vendor.example>",
      "Reply-To: attacker@lookalike.test",
      "Content-Type: text/plain",
      "",
      "Please review your invoice",
    ].join("\r\n");
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "billing@vendor.example",
      subject: "Invoice",
      body: mime,
      messageId: "reply-to-mismatch",
    });
    const [, email] = await emailsRepo.findById("reply-to-mismatch");
    expect(email.headerFrom).toBe("billing@vendor.example");
    expect(email.replyTo).toEqual(["attacker@lookalike.test"]);
    expect(email.securityWarnings).toEqual(["reply_to_mismatch"]);
  });

  it("stores visible To and Cc recipients instead of replacing them with the SMTP envelope", async () => {
    const app = await getApp();
    const mime = [
      "From: Sender <sender@outside.test>",
      "To: Team List <staff@example.net>",
      "Cc: Observer <observer@example.org>, Alice <alice@example.com>",
      "Content-Type: text/plain",
      "",
      "Visible recipients",
    ].join("\r\n");
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "sender@outside.test",
      subject: "Group delivery",
      body: mime,
      messageId: "recipient-headers",
    });
    const [, email] = await emailsRepo.findById("recipient-headers");
    expect(email.recipient).toBe("alice@example.com");
    expect(email.to).toEqual(["staff@example.net"]);
    expect(email.cc).toEqual(["observer@example.org", "alice@example.com"]);
    expect(email.bcc).toBeUndefined();
  });

  it("stores safe List-Unsubscribe methods and RFC 8058 one-click intent", async () => {
    const app = await getApp();
    const mime = [
      "From: Newsletter <news@publisher.example>",
      "List-Unsubscribe: <mailto:leave@publisher.example?subject=unsubscribe>, <https://publisher.example/subscriptions/abc>",
      "List-Unsubscribe-Post: List-Unsubscribe=One-Click",
      "Content-Type: text/plain",
      "",
      "This week's edition",
    ].join("\r\n");
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "news@publisher.example",
      subject: "Weekly edition",
      body: mime,
      messageId: "list-unsubscribe",
    });
    const [, email] = await emailsRepo.findById("list-unsubscribe");
    expect(email.unsubscribeUrl).toBe("https://publisher.example/subscriptions/abc");
    expect(email.unsubscribeMailto).toBe("mailto:leave@publisher.example?subject=unsubscribe");
    expect(email.unsubscribeOneClick).toBe(true);
  });

  it("stores a body unsubscribe link when no header method exists", async () => {
    const app = await getApp();
    const mime = [
      "From: Newsletter <news@publisher.example>",
      "Content-Type: text/html",
      "",
      '<p>Pour vous désabonner, <a href="https://publisher.example/preferences/body-only">cliquez ici</a>.</p>',
    ].join("\r\n");
    await app.inboundDelivery.deliver({
      recipient: "alice@example.com",
      sender: "news@publisher.example",
      subject: "Monthly edition",
      body: mime,
      messageId: "body-unsubscribe",
    });
    const [, email] = await emailsRepo.findById("body-unsubscribe");
    expect(email.unsubscribeBodyUrl).toBe("https://publisher.example/preferences/body-only");
  });
});
