import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import RulesHelper from "@/helpers/RulesHelper.js"
import { Providers } from '../../scripts/support/Providers.mjs'
let fylo;
let rulesRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  rulesRepo = Providers.create(RulesHelper, fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
const rule = {
  id: "rule-001",
  ownerEmail: "alice@example.com",
  mailbox: "alice@example.com",
  domain: "example.com",
  name: "Archive newsletters",
  enabled: true,
  conditionMatch: "any",
  conditions: [
    { field: "subject", op: "contains", value: "newsletter" },
    { field: "from", op: "contains", value: "noreply" }
  ],
  order: 1,
  stopProcessing: false,
  actions: [
    { type: "folder", folder: "newsletters" }
  ]
};
describe("putRule / listRules", () => {
  it("stores and retrieves a rule", async () => {
    await rulesRepo.put(rule);
    const rules = await rulesRepo.list(["example.com"]);
    expect(rules).toHaveLength(1);
    expect(rules[0].name).toBe("Archive newsletters");
  });
  it("deserializes conditions and actions from JSON strings", async () => {
    await rulesRepo.put(rule);
    const rules = await rulesRepo.list(["example.com"]);
    expect(Array.isArray(rules[0].conditions)).toBe(true);
    expect(rules[0].conditions).toHaveLength(2);
    expect(rules[0].conditions[0].field).toBe("subject");
    expect(Array.isArray(rules[0].actions)).toBe(true);
    expect(rules[0].actions[0].type).toBe("folder");
  });
  it("filters rules by allowed domains", async () => {
    await rulesRepo.put(rule);
    await rulesRepo.put({ ...rule, id: "rule-002", domain: "other.com", name: "Other rule" });
    const result = await rulesRepo.list(["example.com"]);
    expect(result).toHaveLength(1);
    expect(result[0].domain).toBe("example.com");
  });
});
describe("listEnabledRulesForDomain", () => {
  it("returns only enabled rules for the domain", async () => {
    const disabled = { ...rule, id: "rule-002", name: "Disabled", enabled: false };
    await rulesRepo.put(rule);
    await rulesRepo.put(disabled);
    const results = await rulesRepo.listEnabledForDomain("example.com");
    expect(results).toHaveLength(1);
    expect(results[0].name).toBe("Archive newsletters");
  });
  it("returns empty when no enabled rules exist", async () => {
    await rulesRepo.put({ ...rule, enabled: false });
    const results = await rulesRepo.listEnabledForDomain("example.com");
    expect(results).toHaveLength(0);
  });
});
describe("owner scope and deterministic order", () => {
  it("returns only the exact owner's rules in order", async () => {
    await rulesRepo.put(rule);
    await rulesRepo.put({ ...rule, id: "rule-002", name: "First", order: 0 });
    await rulesRepo.put({ ...rule, id: "rule-003", ownerEmail: "bob@example.com", mailbox: "bob@example.com" });
    const results = await rulesRepo.listForOwner("ALICE@example.com");
    expect(results.map(item => item.id)).toEqual(["rule-002", "rule-001"]);
  });

  it("restricts mailbox rules while preserving legacy domain rules", async () => {
    await rulesRepo.put(rule);
    await rulesRepo.put({ ...rule, id: "rule-002", ownerEmail: "bob@example.com", mailbox: "bob@example.com" });
    await rulesRepo.put({ ...rule, id: "legacy", ownerEmail: undefined, mailbox: undefined });
    const results = await rulesRepo.listEnabledForMailbox("alice@example.com", "alice@example.com", "example.com");
    expect(results.map(item => item.id)).toEqual(["legacy", "rule-001"]);
  });

  it("reorders the complete exact-owner set and rejects duplicate or partial IDs", async () => {
    await rulesRepo.put(rule);
    await rulesRepo.put({ ...rule, id: "rule-002", name: "Second", order: 2 });
    expect(await rulesRepo.reorder("alice@example.com", ["rule-002", "rule-001"])).toBe(true);
    expect((await rulesRepo.listForOwner("alice@example.com")).map(item => item.id)).toEqual(["rule-002", "rule-001"]);
    expect(await rulesRepo.reorder("alice@example.com", ["rule-001", "rule-001"])).toBe(false);
    expect(await rulesRepo.reorder("alice@example.com", ["rule-001"])).toBe(false);
  });
});
describe("findRuleById", () => {
  it("finds a rule by logical id", async () => {
    await rulesRepo.put(rule);
    const [docId, found] = await rulesRepo.findById("rule-001");
    expect(docId).not.toBeNull();
    expect(found.name).toBe("Archive newsletters");
  });
  it("returns [null, null] when not found", async () => {
    const [docId, found] = await rulesRepo.findById("no-such-rule");
    expect(docId).toBeNull();
    expect(found).toBeNull();
  });
});
describe("updateRule", () => {
  it("updates the name", async () => {
    await rulesRepo.put(rule);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.update(docId, { name: "Renamed rule" });
    const [, updated] = await rulesRepo.findById("rule-001");
    expect(updated.name).toBe("Renamed rule");
  });
  it("updates the enabled flag", async () => {
    await rulesRepo.put(rule);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.update(docId, { enabled: false });
    const [, updated] = await rulesRepo.findById("rule-001");
    expect(updated.enabled).toBe(false);
  });
  it("updates conditions", async () => {
    await rulesRepo.put(rule);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.update(docId, {
      conditions: [{ field: "from", op: "equals", value: "ceo@corp.com" }]
    });
    const [, updated] = await rulesRepo.findById("rule-001");
    expect(updated.conditions).toHaveLength(1);
    expect(updated.conditions[0].value).toBe("ceo@corp.com");
  });
  it("updates actions", async () => {
    await rulesRepo.put(rule);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.update(docId, { actions: [{ type: "delete" }] });
    const [, updated] = await rulesRepo.findById("rule-001");
    expect(updated.actions[0].type).toBe("delete");
  });
});
describe("deleteRule", () => {
  it("removes the rule", async () => {
    await rulesRepo.put(rule);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.delete(docId);
    const rules = await rulesRepo.list(["example.com"]);
    expect(rules).toHaveLength(0);
  });
  it("does not affect other rules", async () => {
    const rule2 = { ...rule, id: "rule-002", name: "Keep me" };
    await rulesRepo.put(rule);
    await rulesRepo.put(rule2);
    const [docId] = await rulesRepo.findById("rule-001");
    await rulesRepo.delete(docId);
    const rules = await rulesRepo.list(["example.com"]);
    expect(rules).toHaveLength(1);
    expect(rules[0].name).toBe("Keep me");
  });
});
