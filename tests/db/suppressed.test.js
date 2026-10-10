import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import { SuppressedRepository } from "@/features/delivery/repositories/SuppressedRepository.js"
let fylo;
let suppressedRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  suppressedRepo = new SuppressedRepository(fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
describe("suppressAddress / listSuppressed", () => {
  it("adds a bounce suppression", async () => {
    await suppressedRepo.suppress("bad@example.com", "bounce");
    const list = await suppressedRepo.list();
    expect(list).toHaveLength(1);
    expect(list[0].address).toBe("bad@example.com");
    expect(list[0].reason).toBe("bounce");
  });
  it("adds a complaint suppression", async () => {
    await suppressedRepo.suppress("spam@example.com", "complaint");
    const list = await suppressedRepo.list();
    expect(list[0].reason).toBe("complaint");
  });
  it("sets suppressedAt to a recent ISO timestamp", async () => {
    const before = Date.now();
    await suppressedRepo.suppress("bad@example.com", "bounce");
    const after = Date.now();
    const [entry] = await suppressedRepo.list();
    const ts = new Date(entry.suppressedAt).getTime();
    expect(ts).toBeGreaterThanOrEqual(before);
    expect(ts).toBeLessThanOrEqual(after);
  });
});
describe("suppressAddress idempotency", () => {
  it("does not duplicate an already suppressed address", async () => {
    await suppressedRepo.suppress("bad@example.com", "bounce");
    await suppressedRepo.suppress("bad@example.com", "bounce");
    await suppressedRepo.suppress("bad@example.com", "complaint");
    const list = await suppressedRepo.list();
    expect(list).toHaveLength(1);
  });
});
describe("getSuppressedSet", () => {
  it("returns a Set of suppressed addresses", async () => {
    await suppressedRepo.suppress("a@example.com", "bounce");
    await suppressedRepo.suppress("b@example.com", "complaint");
    const set = await suppressedRepo.asSet();
    expect(set.size).toBe(2);
    expect(set.has("a@example.com")).toBe(true);
    expect(set.has("b@example.com")).toBe(true);
    expect(set.has("c@example.com")).toBe(false);
  });
  it("returns empty set when no suppressions", async () => {
    const set = await suppressedRepo.asSet();
    expect(set.size).toBe(0);
  });
});
describe("deleteSuppressed", () => {
  it("removes all records for the address", async () => {
    await suppressedRepo.suppress("bad@example.com", "bounce");
    await suppressedRepo.delete("bad@example.com");
    const list = await suppressedRepo.list();
    expect(list).toHaveLength(0);
  });
  it("does not affect other addresses", async () => {
    await suppressedRepo.suppress("a@example.com", "bounce");
    await suppressedRepo.suppress("b@example.com", "bounce");
    await suppressedRepo.delete("a@example.com");
    const list = await suppressedRepo.list();
    expect(list).toHaveLength(1);
    expect(list[0].address).toBe("b@example.com");
  });
  it("is a no-op for addresses not in the list", async () => {
    await suppressedRepo.delete("not-suppressed@example.com");
    const list = await suppressedRepo.list();
    expect(list).toHaveLength(0);
  });
});
