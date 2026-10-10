#!/usr/bin/env bun
import { Collections, DatabaseRepository, collect } from '../server/platform/persistence/fylo.js'
import { DomainsRepository } from '../server/features/domains/repositories/domains.js'
import { DomainMigrationsRepository } from '../server/features/domains/repositories/domain-migrations.js'
import { RulesRepository } from '../server/features/mailbox/repositories/rules.js'
import { UsersRepository } from '../server/features/users/repositories/users.js'
import { Security } from '../server/platform/security/security.js'

const args = parseArgs(process.argv.slice(2))
const fromDomain = Security.normalizeDomain(String(args.from ?? args._[0] ?? ''))
const toDomain = Security.normalizeDomain(String(args.to ?? args._[1] ?? ''))
const apply = args.apply === true || args.apply === 'true'
const dryRun = !apply

if (!fromDomain || !toDomain || fromDomain === toDomain) {
  console.error('Usage: bun scripts/migrate-domain.mjs --from=old.example --to=new.example [--apply]')
  process.exit(1)
}

const fylo = await DatabaseRepository.createDb()
const domainsRepo = new DomainsRepository(fylo)
const usersRepo = new UsersRepository(fylo)
const rulesRepo = new RulesRepository(fylo)
const domainMigrationsRepo = new DomainMigrationsRepository(fylo)

const [, fromConfig] = await domainsRepo.findEntry(fromDomain)
const [, existingToConfig] = await domainsRepo.findEntry(toDomain)
if (!fromConfig) {
  console.error(JSON.stringify({ error: `Source domain not found: ${fromDomain}` }, null, 2))
  await fylo.close()
  process.exit(1)
}

const users = await usersRepo.list()
const primaryMoves = users
  .filter(user => domainOf(user.email) === fromDomain)
  .map(user => ({
    ...user,
    oldEmail: user.email,
    newEmail: `${localPart(user.email)}@${toDomain}`,
  }))

const conflicts = []
for (const move of primaryMoves) {
  const [conflictDocId, conflictUser] = await usersRepo.findByEmail(move.newEmail)
  if (conflictDocId && conflictDocId !== move.docId) {
    conflicts.push({
      oldEmail: move.oldEmail,
      newEmail: move.newEmail,
      conflictEmail: conflictUser?.email,
    })
  }
}

if (conflicts.length > 0) {
  console.error(JSON.stringify({ error: 'Migration has conflicting destination users', conflicts }, null, 2))
  await fylo.close()
  process.exit(1)
}

const domainAction = existingToConfig ? 'kept-existing' : 'created-from-source'
const affectedDomainUsers = users
  .filter(user => user.domains.includes(fromDomain) && !primaryMoves.some(move => move.docId === user.docId))
  .map(user => user.email)
const rulesToCopy = await rulesRepo.list([fromDomain])
const plannedRules = []

for (const rule of rulesToCopy) {
  const id = `${rule.id}-migrated-${toDomain}`
  const [existingRuleDocId] = await rulesRepo.findById(id)
  plannedRules.push({
    from: rule.id,
    to: id,
    action: existingRuleDocId ? 'kept-existing' : 'created-from-source',
  })
}

const plan = {
  dryRun,
  fromDomain,
  toDomain,
  domain: domainAction,
  usersPromoted: primaryMoves.map(move => ({ from: move.oldEmail, to: move.newEmail })),
  usersGrantedNewDomain: affectedDomainUsers,
  inboxRules: plannedRules,
}

if (dryRun) {
  console.log(JSON.stringify(plan, null, 2))
  await fylo.close()
  process.exit(0)
}

if (!existingToConfig) {
  await domainsRepo.put({
    ...fromConfig,
    domain: toDomain,
    routes: fromConfig.routes.map(route => rewriteRouteForDomain(route, fromDomain, toDomain)),
  })
}

for (const rule of rulesToCopy) {
  const id = `${rule.id}-migrated-${toDomain}`
  const [existingRuleDocId] = await rulesRepo.findById(id)
  if (existingRuleDocId) continue
  await rulesRepo.put({
    ...rule,
    id,
    domain: toDomain,
  })
}

for (const move of primaryMoves) {
  const updated = /** @type {any} */ (UsersRepository.normalize({
    ...move,
    email: move.newEmail,
    aliases: [...(move.aliases ?? []), move.oldEmail],
    domains: [...move.domains, fromDomain, toDomain],
  }))
  const { docId, oldEmail, newEmail, ...user } = updated
  void docId
  void oldEmail
  void newEmail
  await fylo.patchDoc(Collections.USERS, { [move.docId]: user })
  await rewriteUserEmailReferences(fylo, move.oldEmail, move.newEmail)
}

for (const user of users) {
  if (!user.domains.includes(fromDomain)) continue
  if (primaryMoves.some(move => move.docId === user.docId)) continue
  const updated = /** @type {any} */ (UsersRepository.normalize({
    ...user,
    domains: [...user.domains, toDomain],
  }))
  const { docId, ...patch } = updated
  await fylo.patchDoc(Collections.USERS, { [docId]: patch })
}

await domainMigrationsRepo.put({
  id: `domain-migration:${fromDomain}->${toDomain}`,
  fromDomain,
  toDomain,
  mode: 'alias',
  localPartStrategy: 'preserve',
  createdAt: new Date().toISOString(),
  appliedAt: new Date().toISOString(),
})

console.log(JSON.stringify({ ...plan, dryRun: false, applied: true }, null, 2))
await fylo.close()

function parseArgs(argv) {
  const parsed = /** @type {Record<string, any> & { _: string[] }} */ ({ _: [] })
  for (const arg of argv) {
    if (!arg.startsWith('--')) {
      parsed._.push(arg)
      continue
    }
    const [key, ...rest] = arg.slice(2).split('=')
    parsed[key] = rest.length ? rest.join('=') : true
  }
  return parsed
}

function domainOf(email) {
  return email.split('@')[1]?.toLowerCase() ?? ''
}

function localPart(email) {
  return email.split('@')[0].toLowerCase()
}

function rewriteRouteForDomain(route, fromDomain, toDomain) {
  return {
    ...route,
    id: route.id === `store-${fromDomain}` ? `store-${toDomain}` : route.id,
    match: rewriteAddressPattern(route.match, fromDomain, toDomain),
  }
}

function rewriteAddressPattern(value, fromDomain, toDomain) {
  if (value === '*') return value
  if (value === `*@${fromDomain}`) return `*@${toDomain}`
  if (value.endsWith(`@${fromDomain}`)) return `${value.slice(0, -fromDomain.length)}${toDomain}`
  return value
}

async function rewriteUserEmailReferences(fylo, oldEmail, newEmail) {
  await rewriteCollectionField(fylo, Collections.MFA_SESSIONS, 'email', oldEmail, newEmail)
  await rewriteCollectionField(fylo, Collections.SETUP_SESSIONS, 'email', oldEmail, newEmail)
  await rewriteCollectionField(fylo, Collections.PUSH_SUBSCRIPTIONS, 'userEmail', oldEmail, newEmail)
  await rewriteCollectionField(fylo, Collections.FOLLOW_UP_REMINDERS, 'ownerEmail', oldEmail, newEmail)
}

async function rewriteCollectionField(fylo, collection, field, oldValue, newValue) {
  const docs = await collect(
    fylo.findDocs(collection, {
      $ops: [{ [field]: { $eq: oldValue } }],
    }).collect()
  )
  await Promise.all(
    Object.keys(docs).map(docId => fylo.patchDoc(collection, { [docId]: { [field]: newValue } }))
  )
}
