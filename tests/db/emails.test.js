import { describe, it, expect, beforeEach, afterEach } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { DatabaseRepository } from "@/platform/persistence/Fylo.js";
import { EmailsRepository } from "@/features/mailbox/repositories/EmailsRepository.js";
import { AttachmentsRepository } from "@/features/mailbox/repositories/AttachmentsRepository.js";
let fylo;
let emailsRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  emailsRepo = new EmailsRepository(fylo, new AttachmentsRepository(fylo));
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
function makeEmail(overrides = {}) {
  return {
    id: "msg-001",
    domain: "example.com",
    recipient: "alice@example.com",
    sender: "sender@other.com",
    subject: "Hello",
    body: "Hi there!",
    folder: "inbox",
    read: false,
    starred: false,
    receivedAt: "2024-01-01T10:00:00.000Z",
    processed: false,
    ...overrides
  };
}
describe("putEmail / listEmails", () => {
  it("stores and retrieves an email", async () => {
    await emailsRepo.put(makeEmail());
    const emails = await emailsRepo.list(["example.com"]);
    expect(emails).toHaveLength(1);
    expect(emails[0].subject).toBe("Hello");
    expect(emails[0].sender).toBe("sender@other.com");
  });
  it("filters emails by allowed domains", async () => {
    await emailsRepo.put(makeEmail({ domain: "example.com" }));
    await emailsRepo.put(makeEmail({ id: "msg-002", domain: "other.com" }));
    const result = await emailsRepo.list(["example.com"]);
    expect(result).toHaveLength(1);
    expect(result[0].domain).toBe("example.com");
  });
  it("returns empty array when no allowed domains match", async () => {
    await emailsRepo.put(makeEmail());
    const emails = await emailsRepo.list(["nowhere.com"]);
    expect(emails).toHaveLength(0);
  });
  it("sorts emails newest-first", async () => {
    await emailsRepo.put(makeEmail({ id: "msg-001", receivedAt: "2024-01-01T10:00:00.000Z" }));
    await emailsRepo.put(makeEmail({ id: "msg-002", receivedAt: "2024-01-02T10:00:00.000Z" }));
    await emailsRepo.put(makeEmail({ id: "msg-003", receivedAt: "2024-01-03T10:00:00.000Z" }));
    const emails = await emailsRepo.list(["example.com"]);
    expect(emails[0].id).toBe("msg-003");
    expect(emails[2].id).toBe("msg-001");
  });
  it("searches sender, subject, and body", async () => {
    await emailsRepo.put(makeEmail({
      id: "msg-invoice",
      sender: "billing@vendor.com",
      subject: "April invoice",
      body: "Total due Friday"
    }));
    await emailsRepo.put(makeEmail({
      id: "msg-memo",
      sender: "notes@vendor.com",
      subject: "Team memo",
      body: "Lunch plan"
    }));
    const bySender = await emailsRepo.list(["example.com"], { query: "from:billing" });
    expect(bySender.map((e) => e.id)).toEqual(["msg-invoice"]);
    const byBody = await emailsRepo.list(["example.com"], { query: "friday" });
    expect(byBody.map((e) => e.id)).toEqual(["msg-invoice"]);
  });
  it("filters by folder, read state, starred state, and pagination", async () => {
    await emailsRepo.put(makeEmail({ id: "msg-001", folder: "inbox", read: false, starred: true, receivedAt: "2024-01-01T10:00:00.000Z" }));
    await emailsRepo.put(makeEmail({ id: "msg-002", folder: "archive", read: true, starred: false, receivedAt: "2024-01-02T10:00:00.000Z" }));
    await emailsRepo.put(makeEmail({ id: "msg-003", folder: "trash", read: true, starred: true, receivedAt: "2024-01-03T10:00:00.000Z" }));
    const unread = await emailsRepo.list(["example.com"], { read: false });
    expect(unread.map((e) => e.id)).toEqual(["msg-001"]);
    const archive = await emailsRepo.list(["example.com"], { folder: "archive" });
    expect(archive.map((e) => e.id)).toEqual(["msg-002"]);
    const starred = await emailsRepo.list(["example.com"], { starred: true, offset: 1, limit: 1 });
    expect(starred.map((e) => e.id)).toEqual(["msg-001"]);
  });
});
describe("findEmailById", () => {
  it("finds an email by logical id", async () => {
    await emailsRepo.put(makeEmail({ id: "unique-id-42" }));
    const [docId, email] = await emailsRepo.findById("unique-id-42");
    expect(docId).not.toBeNull();
    expect(email).not.toBeNull();
    expect(email.id).toBe("unique-id-42");
  });
  it("returns [null, null] when not found", async () => {
    const [docId, email] = await emailsRepo.findById("does-not-exist");
    expect(docId).toBeNull();
    expect(email).toBeNull();
  });
});
describe("updateEmail", () => {
  it("patches the folder field", async () => {
    await emailsRepo.put(makeEmail());
    const [docId] = await emailsRepo.findById("msg-001");
    await emailsRepo.update(docId, { folder: "archive" });
    const [, updated] = await emailsRepo.findById("msg-001");
    expect(updated.folder).toBe("archive");
  });
  it("patches the processed flag", async () => {
    await emailsRepo.put(makeEmail({ processed: false }));
    const [docId] = await emailsRepo.findById("msg-001");
    await emailsRepo.update(docId, { processed: true });
    const [, updated] = await emailsRepo.findById("msg-001");
    expect(updated.processed).toBe(true);
  });
  it("patches read and starred flags", async () => {
    await emailsRepo.put(makeEmail({ read: false, starred: false }));
    const [docId] = await emailsRepo.findById("msg-001");
    await emailsRepo.update(docId, { read: true, starred: true });
    const [, updated] = await emailsRepo.findById("msg-001");
    expect(updated.read).toBe(true);
    expect(updated.starred).toBe(true);
  });
});
describe("deleteEmail", () => {
  it("removes the email", async () => {
    await emailsRepo.put(makeEmail());
    const [docId] = await emailsRepo.findById("msg-001");
    await emailsRepo.delete(docId);
    const emails = await emailsRepo.list(["example.com"]);
    expect(emails).toHaveLength(0);
  });
  it("does not affect other emails", async () => {
    await emailsRepo.put(makeEmail({ id: "msg-001" }));
    await emailsRepo.put(makeEmail({ id: "msg-002" }));
    const [docId] = await emailsRepo.findById("msg-001");
    await emailsRepo.delete(docId);
    const emails = await emailsRepo.list(["example.com"]);
    expect(emails).toHaveLength(1);
    expect(emails[0].id).toBe("msg-002");
  });
});
