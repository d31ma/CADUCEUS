import { describe, it, expect, beforeEach, afterEach } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { DatabaseRepository } from "@/shared/persistence/Fylo.js";
import { PushSubscriptionsRepository } from "@/features/notifications/repositories/PushSubscriptionsRepository.js";
let fylo;
let pushRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-push-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  pushRepo = new PushSubscriptionsRepository(fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
const sampleSubscription = {
  userEmail: "Alice@Example.com",
  endpoint: "https://push.example.test/subscription/1",
  keys: {
    p256dh: "p256dh-key",
    auth: "auth-key"
  },
  userAgent: "test-browser"
};
describe("push subscriptions", () => {
  it("stores subscriptions with stable endpoint IDs", async () => {
    const stored = await pushRepo.upsert(sampleSubscription);
    expect(stored.id).toBe(PushSubscriptionsRepository.subscriptionId(sampleSubscription.endpoint));
    expect(stored.userEmail).toBe("alice@example.com");
    const [docId, found] = await pushRepo.findById(stored.id);
    expect(docId).not.toBeNull();
    expect(found?.endpoint).toBe(sampleSubscription.endpoint);
  });
  it("lists subscriptions by user address case-insensitively", async () => {
    await pushRepo.upsert(sampleSubscription);
    await pushRepo.upsert({
      ...sampleSubscription,
      userEmail: "bob@example.com",
      endpoint: "https://push.example.test/subscription/2"
    });
    const byUser = await pushRepo.listForUser("alice@example.com");
    const byAddress = await pushRepo.listForAddress("ALICE@EXAMPLE.COM");
    expect(byUser).toHaveLength(1);
    expect(byAddress).toHaveLength(1);
    expect(byAddress[0].endpoint).toBe(sampleSubscription.endpoint);
  });
  it("updates an existing endpoint instead of duplicating it", async () => {
    const first = await pushRepo.upsert(sampleSubscription);
    const second = await pushRepo.upsert({
      ...sampleSubscription,
      userAgent: "updated-browser"
    });
    const subscriptions = await pushRepo.listForUser("alice@example.com");
    expect(second.id).toBe(first.id);
    expect(second.createdAt).toBe(first.createdAt);
    expect(subscriptions).toHaveLength(1);
    expect(subscriptions[0].userAgent).toBe("updated-browser");
  });
  it("deletes subscriptions by endpoint", async () => {
    await pushRepo.upsert(sampleSubscription);
    const deleted = await pushRepo.deleteByEndpoint(sampleSubscription.endpoint);
    const remaining = await pushRepo.listForUser("alice@example.com");
    expect(deleted).toBe(true);
    expect(remaining).toHaveLength(0);
  });
});
