import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import UsersHelper from "@/helpers/UsersHelper.js"
import { Providers } from '../../scripts/support/Providers.mjs'
let fylo;
let usersRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-test-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  usersRepo = Providers.create(UsersHelper, fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
const alice = {
  email: "alice@example.com",
  aliases: [],
  domains: ["example.com"],
  role: "admin"
};
const bob = {
  email: "bob@example.com",
  aliases: [],
  domains: ["example.com"],
  role: "viewer"
};
describe("putUser / listUsers", () => {
  it("stores and retrieves a user", async () => {
    await usersRepo.put(alice);
    const users = await usersRepo.list();
    expect(users).toHaveLength(1);
    expect(users[0].email).toBe("alice@example.com");
    expect(users[0].role).toBe("admin");
  });
  it("normalises email to lowercase on insert", async () => {
    await usersRepo.put({ ...alice, email: "ALICE@Example.COM" });
    const [, user] = await usersRepo.findByEmail("alice@example.com");
    expect(user).not.toBeNull();
    expect(user.email).toBe("alice@example.com");
  });
  it("stores multiple users", async () => {
    await usersRepo.put(alice);
    await usersRepo.put(bob);
    const users = await usersRepo.list();
    expect(users).toHaveLength(2);
  });
});
describe("findUserByEmail", () => {
  it("returns the user when found", async () => {
    await usersRepo.put(alice);
    const [docId, user] = await usersRepo.findByEmail("alice@example.com");
    expect(docId).not.toBeNull();
    expect(user).not.toBeNull();
    expect(user.email).toBe("alice@example.com");
  });
  it("returns [null, null] when not found", async () => {
    const [docId, user] = await usersRepo.findByEmail("nobody@example.com");
    expect(docId).toBeNull();
    expect(user).toBeNull();
  });
  it("finds user case-insensitively", async () => {
    await usersRepo.put(alice);
    const [, user] = await usersRepo.findByEmail("ALICE@EXAMPLE.COM");
    expect(user).not.toBeNull();
  });
  it("finds user by alias", async () => {
    await usersRepo.put({ ...alice, aliases: ["alice@old.example.com"] });
    const [, user] = await usersRepo.findByEmail("ALICE@OLD.EXAMPLE.COM");
    expect(user).not.toBeNull();
    expect(user.email).toBe("alice@example.com");
  });
  it("updates aliases independently from the rest of the user", async () => {
    await usersRepo.put(alice);
    const [docId] = await usersRepo.findByEmail(alice.email);
    await usersRepo.updateAliases(docId, ["Alice+sales@Example.com", alice.email], alice.email);
    const [, user] = await usersRepo.findByEmail("alice+sales@example.com");
    expect(user.email).toBe(alice.email);
    expect(user.aliases).toEqual(["alice+sales@example.com"]);
  });
});
describe("deleteUser", () => {
  it("removes the user", async () => {
    await usersRepo.put(alice);
    const [docId] = await usersRepo.findByEmail(alice.email);
    expect(docId).not.toBeNull();
    await usersRepo.delete(docId);
    const users = await usersRepo.list();
    expect(users).toHaveLength(0);
  });
  it("does not affect other users", async () => {
    await usersRepo.put(alice);
    await usersRepo.put(bob);
    const [aliceDocId] = await usersRepo.findByEmail(alice.email);
    await usersRepo.delete(aliceDocId);
    const [, remaining] = await usersRepo.findByEmail(bob.email);
    expect(remaining).not.toBeNull();
  });
});
