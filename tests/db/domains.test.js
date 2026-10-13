import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import DomainsHelper from "@/helpers/DomainsHelper.js"
import { Providers } from '../../scripts/support/Providers.mjs'
let fylo;
let domainsRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  domainsRepo = Providers.create(DomainsHelper, fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
const exampleDomain = {
  domain: "example.com",
  routes: [
    { id: "r1", match: "*@example.com", action: { type: "store" }, enabled: true }
  ],
  inboundEnabled: true
};
const otherDomain = {
  domain: "other.com",
  routes: [],
  inboundEnabled: false
};
describe("putDomain / listDomains", () => {
  it("stores and retrieves a domain config", async () => {
    await domainsRepo.put(exampleDomain);
    const domains = await domainsRepo.list(["example.com"]);
    expect(domains).toHaveLength(1);
    expect(domains[0].domain).toBe("example.com");
    expect(domains[0].inboundEnabled).toBe(true);
  });
  it("deserializes routes from JSON strings", async () => {
    await domainsRepo.put(exampleDomain);
    const [, config] = await domainsRepo.findEntry("example.com");
    expect(Array.isArray(config.routes)).toBe(true);
    expect(config.routes[0].id).toBe("r1");
    expect(config.routes[0].action.type).toBe("store");
  });
  it("filters domains by allowed list", async () => {
    await domainsRepo.put(exampleDomain);
    await domainsRepo.put(otherDomain);
    const result = await domainsRepo.list(["example.com"]);
    expect(result).toHaveLength(1);
    expect(result[0].domain).toBe("example.com");
  });
  it("returns empty array when no match", async () => {
    await domainsRepo.put(exampleDomain);
    const result = await domainsRepo.list(["nowhere.com"]);
    expect(result).toHaveLength(0);
  });
});
describe("findDomainEntry", () => {
  it("returns [docId, config] when found", async () => {
    await domainsRepo.put(exampleDomain);
    const [docId, config] = await domainsRepo.findEntry("example.com");
    expect(docId).not.toBeNull();
    expect(config).not.toBeNull();
    expect(config.domain).toBe("example.com");
  });
  it("returns [null, null] when not found", async () => {
    const [docId, config] = await domainsRepo.findEntry("nope.com");
    expect(docId).toBeNull();
    expect(config).toBeNull();
  });
});
describe("updateDomainRoutes", () => {
  it("replaces the routes array", async () => {
    await domainsRepo.put(exampleDomain);
    const [docId] = await domainsRepo.findEntry("example.com");
    const newRoutes = [
      { id: "r2", match: "admin@example.com", action: { type: "forward", to: "me@other.com" }, enabled: true },
      { id: "r3", match: "*", action: { type: "drop" }, enabled: true }
    ];
    await domainsRepo.updateRoutes(docId, newRoutes);
    const [, updated] = await domainsRepo.findEntry("example.com");
    expect(updated.routes).toHaveLength(2);
    expect(updated.routes[0].id).toBe("r2");
    expect(updated.routes[1].action.type).toBe("drop");
  });
  it("serializes webhook action with optional secret", async () => {
    await domainsRepo.put(exampleDomain);
    const [docId] = await domainsRepo.findEntry("example.com");
    const routes = [
      {
        id: "wh1",
        match: "*@example.com",
        action: { type: "webhook", url: "https://hooks.example.com/endpoint", secret: "mysecret" },
        enabled: true
      }
    ];
    await domainsRepo.updateRoutes(docId, routes);
    const [, updated] = await domainsRepo.findEntry("example.com");
    const action = updated.routes[0].action;
    expect(action.type).toBe("webhook");
    expect(action.url).toBe("https://hooks.example.com/endpoint");
    expect(action.secret).toBe("mysecret");
  });
});
describe("deleteDomain", () => {
  it("removes the domain", async () => {
    await domainsRepo.put(exampleDomain);
    const [docId] = await domainsRepo.findEntry("example.com");
    await domainsRepo.delete(docId);
    const [, config] = await domainsRepo.findEntry("example.com");
    expect(config).toBeNull();
  });
});
