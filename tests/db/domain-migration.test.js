import { describe, it, expect, beforeEach, afterEach } from "bun:test"
import { mkdtempSync, rmSync } from "node:fs"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { DatabaseRepository } from "@/shared/persistence/DatabaseRepository.js"
import { DomainsRepository } from "@/features/domains/repositories/DomainsRepository.js"
import { DomainMigrationsRepository } from "@/features/domains/repositories/DomainMigrationsRepository.js"
import { EmailsRepository } from "@/features/mailbox/repositories/EmailsRepository.js"
import { AttachmentsRepository } from "@/features/mailbox/repositories/AttachmentsRepository.js"
import { UsersRepository } from "@/features/users/repositories/UsersRepository.js"
import { PushSubscriptionsRepository } from "@/features/notifications/repositories/PushSubscriptionsRepository.js"
import { DomainMigrationService } from "@/features/domains/services/DomainMigrationService.js"
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
  emailsRepo = new EmailsRepository(fylo, new AttachmentsRepository(fylo));
  usersRepo = new UsersRepository(fylo);
  domainsRepo = new DomainsRepository(fylo);
  pushRepo = new PushSubscriptionsRepository(fylo);
  domainMigrationsRepo = new DomainMigrationsRepository(fylo);
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
    emailsRepo = new EmailsRepository(fylo, new AttachmentsRepository(fylo));
    usersRepo = new UsersRepository(fylo);
    domainsRepo = new DomainsRepository(fylo);
    pushRepo = new PushSubscriptionsRepository(fylo);
    domainMigrationsRepo = new DomainMigrationsRepository(fylo);
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
    domainMigrationsRepo = new DomainMigrationsRepository(fylo);
    emailsRepo = new EmailsRepository(fylo, new AttachmentsRepository(fylo));
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
    const presented = await new DomainMigrationService(domainMigrationsRepo).presentEmail(storedEmail);
    expect(presented.recipient).toBe("alice@new.example");
    expect(presented.originalRecipient).toBe("alice@old.example");
  });
});
