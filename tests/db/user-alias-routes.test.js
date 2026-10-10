import { describe, it, expect, beforeAll, beforeEach, afterAll } from "bun:test";
import { setupRoutes, teardownRoutes, token as issue } from "../routes/helpers.js";
import { Routes } from "../support/Routes.mjs";
const AliasesHandler = Routes.UsersAliasesController;
import { call } from "../support/controller-call.mjs";

const getAliases = (req) => call(AliasesHandler, 'GET', req);
const addAlias = (req) => call(AliasesHandler, 'POST', req);
const removeAlias = (req) => call(AliasesHandler, 'DELETE', req);

let app;
let usersRepo;
let token;

beforeAll(async () => {
  app = await setupRoutes();
  usersRepo = app.users;
  token = issue();
}, 300_000);
afterAll(teardownRoutes);

beforeEach(async () => {
  // Each test starts from one alias-free mailbox record.
  for (const alias of ["alice+sales@example.com"]) {
    const [docId] = await usersRepo.findByEmail(alias);
    if (docId) await usersRepo.delete(docId);
  }
  await usersRepo.put({
    email: "alice@example.com",
    aliases: [],
    domains: ["example.com"],
    role: "admin",
  });
});

describe("user aliases API", () => {
  it("returns aliases for the authenticated user", async () => {
    await addAlias({ body: { alias: "alice+sales@example.com" }, context: { bearer: { token } } });
    const res = await getAliases({ context: { bearer: { token } } });
    expect(res).toEqual({ email: "alice@example.com", aliases: ["alice+sales@example.com"] });
  });

  it("adds an alias to the authenticated user", async () => {
    const res = await addAlias({ body: { alias: "Alice+Sales@Example.com" }, context: { bearer: { token } } });
    expect(res.aliases).toEqual(["alice+sales@example.com"]);
    const [, user] = await usersRepo.findByEmail("alice+sales@example.com");
    expect(user.email).toBe("alice@example.com");
  });

  it("rejects aliases outside the user's domains", async () => {
    const res = await addAlias({ body: { alias: "alice@other.example" }, context: { bearer: { token } } });
    expect(res.forbidden).toBe(true);
  });

  it("removes an alias from the authenticated user", async () => {
    await addAlias({ body: { alias: "alice+sales@example.com" }, context: { bearer: { token } } });
    const res = await removeAlias({ body: { alias: "alice+sales@example.com" }, context: { bearer: { token } } });
    expect(res.aliases).toEqual([]);
    const [, user] = await usersRepo.findByEmail("alice+sales@example.com");
    expect(user).toBeNull();
  });
});
