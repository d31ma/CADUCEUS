import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import DomainsHelper from "@/helpers/DomainsHelper.js"
import DomainMigrationsHelper from "@/helpers/DomainMigrationsHelper.js"
import EmailsHelper from "@/helpers/EmailsHelper.js"
import AttachmentsHelper from "@/helpers/AttachmentsHelper.js"
import UsersHelper from "@/helpers/UsersHelper.js"
import PushSubscriptionsHelper from "@/helpers/PushSubscriptionsHelper.js"
import DomainMigrationService from "@/services/DomainMigrationService.js"
import { Providers } from '../../scripts/support/Providers.mjs'
let fylo;
let emailsRepo;
let usersRepo;
let domainsRepo;
let pushRepo;
let domainMigrationsRepo;
let testRoot;
beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), "caduceus-domain-migration-"));
  fylo = await DatabaseRepository.createDb(testRoot);
  emailsRepo = Providers.create(EmailsHelper, fylo, Providers.create(AttachmentsHelper, fylo));
  usersRepo = Providers.create(UsersHelper, fylo);
  domainsRepo = Providers.create(DomainsHelper, fylo);
  pushRepo = Providers.create(PushSubscriptionsHelper, fylo);
  domainMigrationsRepo = Providers.create(DomainMigrationsHelper, fylo);
});
afterEach(async () => {
  await fylo?.close?.();
  rmSync(testRoot, { recursive: true, force: true });
});
describe("domain:migrate", () => {
  it("promotes old-domain users to the new suffix while preserving aliases and owned records", async () => {
    await domainsRepo.put({
      domain: "old.example",
      inboundEnabled: true,
      routes: [{ id: "store-old.example", match: "*@old.example", action: { type: "store" }, enabled: true }]
    });
    await usersRepo.put({
      email: "alice@old.example",
      domains: ["old.example"],
      role: "admin"
    });
    await pushRepo.upsert({
      userEmail: "alice@old.example",
      endpoint: "https://push.example.test/alice",
      keys: { p256dh: "p256dh", auth: "auth" }
    });
    await fylo.close();
    const dryRun = Bun.spawnSync([
      "bun",
      "scripts/migrate-domain.mjs",
      "--from=old.example",
      "--to=new.example"
    ], {
      cwd: join(import.meta.dir, "..", ".."),
      env: { ...process.env, FYLO_ROOT: testRoot },
      stdout: "pipe",
      stderr: "pipe"
    });
    expect(dryRun.exitCode).toBe(0);
    expect(JSON.parse(dryRun.stdout.toString()).dryRun).toBe(true);
    const applied = Bun.spawnSync([
      "bun",
      "scripts/migrate-domain.mjs",
      "--from=old.example",
      "--to=new.example",
      "--apply"
    ], {
      cwd: join(import.meta.dir, "..", ".."),
      env: { ...process.env, FYLO_ROOT: testRoot },
      stdout: "pipe",
      stderr: "pipe"
    });
    expect(applied.exitCode).toBe(0);
    fylo = await DatabaseRepository.createDb(testRoot);
    emailsRepo = Providers.create(EmailsHelper, fylo, Providers.create(AttachmentsHelper, fylo));
    usersRepo = Providers.create(UsersHelper, fylo);
    domainsRepo = Providers.create(DomainsHelper, fylo);
    pushRepo = Providers.create(PushSubscriptionsHelper, fylo);
    domainMigrationsRepo = Providers.create(DomainMigrationsHelper, fylo);
    const [, newDomain] = await domainsRepo.findEntry("new.example");
    expect(newDomain).not.toBeNull();
    expect(newDomain.routes[0].match).toBe("*@new.example");
    const [, byNewEmail] = await usersRepo.findByEmail("alice@new.example");
    expect(byNewEmail).not.toBeNull();
    expect(byNewEmail.email).toBe("alice@new.example");
    expect(byNewEmail.aliases).toContain("alice@old.example");
    expect(byNewEmail.domains).toContain("old.example");
    expect(byNewEmail.domains).toContain("new.example");
    const [, byOldAlias] = await usersRepo.findByEmail("alice@old.example");
    expect(byOldAlias.email).toBe("alice@new.example");
    expect(await pushRepo.listForUser("alice@old.example")).toHaveLength(0);
    expect(await pushRepo.listForUser("alice@new.example")).toHaveLength(1);
    const migrations = await domainMigrationsRepo.list();
    expect(migrations.some((migration) => migration.fromDomain === "old.example" && migration.toDomain === "new.example")).toBe(true);
    await fylo.close();
    const retried = Bun.spawnSync([
      "bun",
      "scripts/migrate-domain.mjs",
      "--from=old.example",
      "--to=new.example",
      "--apply"
    ], {
      cwd: join(import.meta.dir, "..", ".."),
      env: { ...process.env, FYLO_ROOT: testRoot },
      stdout: "pipe",
      stderr: "pipe"
    });
    expect(retried.exitCode).toBe(0);
    fylo = await DatabaseRepository.createDb(testRoot);
    domainMigrationsRepo = Providers.create(DomainMigrationsHelper, fylo);
    emailsRepo = Providers.create(EmailsHelper, fylo, Providers.create(AttachmentsHelper, fylo));
    expect((await domainMigrationsRepo.list()).filter(
      migration => migration.fromDomain === "old.example" && migration.toDomain === "new.example"
    )).toHaveLength(1);
    await emailsRepo.put({
      id: "old-mail-1",
      domain: "old.example",
      recipient: "alice@old.example",
      sender: "sender@example.test",
      subject: "Before migration",
      body: "Historical mail",
      folder: "inbox",
      read: false,
      starred: false,
      receivedAt: new Date().toISOString(),
      processed: true
    });
    const [, storedEmail] = await emailsRepo.findById("old-mail-1");
    const presented = await Providers.create(DomainMigrationService, domainMigrationsRepo).presentEmail(storedEmail);
    expect(presented.recipient).toBe("alice@new.example");
    expect(presented.originalRecipient).toBe("alice@old.example");
  });
});
