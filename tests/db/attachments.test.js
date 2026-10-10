import { describe, it, expect, beforeEach, afterEach } from "bun:test";
import { mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { Buffer } from "node:buffer";
import { writeFileSync } from "node:fs";
import { DatabaseRepository, Buckets } from "@/shared/persistence/Fylo.js";
import { AttachmentsRepository } from "@/features/mailbox/repositories/AttachmentsRepository.js";
let fylo;
let attachmentsRepo;
let testRoot;
let previousFyloRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-attachments-test-"));
  previousFyloRoot = process.env.FYLO_ROOT;
  process.env.FYLO_ROOT = testRoot;
  fylo = await DatabaseRepository.createDb(testRoot);
  attachmentsRepo = new AttachmentsRepository(fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  if (previousFyloRoot === undefined)
    delete process.env.FYLO_ROOT;
  else
    process.env.FYLO_ROOT = previousFyloRoot;
  rmSync(testRoot, { recursive: true, force: true });
});
describe("email attachment storage", () => {
  it("saves metadata and file content", async () => {
    const summaries = await attachmentsRepo.saveForEmail("msg-1", "example.com", [
      {
        filename: "hello.txt",
        contentType: "text/plain",
        content: new TextEncoder().encode("hello attachment"),
        disposition: "attachment"
      }
    ]);
    expect(summaries).toHaveLength(1);
    expect(summaries[0].filename).toBe("hello.txt");
    expect(summaries[0].size).toBe("hello attachment".length);
    const [docId, record] = await attachmentsRepo.findById(summaries[0].id);
    expect(docId).not.toBeNull();
    expect(record?.emailId).toBe("msg-1");
    expect(record?.bucketId).toBeString();
    expect(record?.storagePath).toBeUndefined();
    const stored = await fylo.getDoc(Buckets.ATTACHMENT_FILES, record.bucketId);
    expect(stored[record.bucketId].contentLength).toBe("hello attachment".length);
    expect(stored[record.bucketId].key).toContain("/emails/");
    expect(new TextDecoder().decode(await attachmentsRepo.readContent(record))).toBe("hello attachment");
  });
  it("saves outbound base64 attachment content", async () => {
    const summaries = await attachmentsRepo.saveForEmail("sent-1", "example.com", [
      {
        filename: "sent.txt",
        contentType: "text/plain",
        contentBase64: Buffer.from("outbound attachment").toString("base64")
      }
    ]);
    const [, record] = await attachmentsRepo.findById(summaries[0].id);
    expect(record?.emailId).toBe("sent-1");
    expect(new TextDecoder().decode(await attachmentsRepo.readContent(record))).toBe("outbound attachment");
  });
  it("normalizes untrusted filenames, content types, and Content-IDs before persistence", async () => {
    const [summary] = await attachmentsRepo.saveForEmail("unsafe-1", "example.com", [
      {
        filename: "../../invoice\u202Efdp.exe",
        contentType: "IMAGE/PNG; name=ignored.png",
        contentId: "cid:<Logo@Example.test>",
        content: new Uint8Array([0])
      }
    ]);
    expect(summary).toMatchObject({
      filename: "invoicefdp.exe",
      contentType: "image/png",
      contentId: "Logo@Example.test"
    });
  });
  it("stores empty attachments in the bucket", async () => {
    const [summary] = await attachmentsRepo.saveForEmail("empty-1", "example.com", [
      { filename: "empty.txt", contentType: "text/plain", content: new Uint8Array() }
    ]);
    const [, record] = await attachmentsRepo.findById(summary.id);
    expect(summary.size).toBe(0);
    expect(await attachmentsRepo.readContent(record)).toHaveLength(0);
  });
  it("lists summaries without storage paths", async () => {
    await attachmentsRepo.saveForEmail("msg-1", "example.com", [
      { filename: "a.txt", contentType: "text/plain", content: new TextEncoder().encode("a") }
    ]);
    const summaries = await attachmentsRepo.listSummaries("msg-1");
    expect(summaries).toHaveLength(1);
    expect(summaries[0].storagePath).toBeUndefined();
  });
  it("deletes metadata and files for an email", async () => {
    const [summary] = await attachmentsRepo.saveForEmail("msg-1", "example.com", [
      { filename: "gone.txt", contentType: "text/plain", content: new TextEncoder().encode("gone") }
    ]);
    const [, storedRecord] = await attachmentsRepo.findById(summary.id);
    await attachmentsRepo.deleteForEmail("msg-1");
    const [, record] = await attachmentsRepo.findById(summary.id);
    expect(record).toBeNull();
    await expect(fylo.getFileBytes(Buckets.ATTACHMENT_FILES, storedRecord.bucketId)).rejects.toThrow();
  });
  it("continues reading legacy path-backed attachment records", async () => {
    const legacyRoot = join(testRoot, "attachments");
    mkdirSync(legacyRoot, { recursive: true });
    const legacyPath = join(legacyRoot, "legacy-attachment.txt");
    writeFileSync(legacyPath, "legacy bytes");
    expect(new TextDecoder().decode(await attachmentsRepo.readContent({ storagePath: legacyPath }))).toBe("legacy bytes");
  });
});
