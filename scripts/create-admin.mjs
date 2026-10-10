#!/usr/bin/env bun
import { DatabaseRepository } from '../server/shared/persistence/DatabaseRepository.js'
import { DomainsRepository } from '../server/features/domains/repositories/DomainsRepository.js'
import { UsersRepository } from '../server/features/users/repositories/UsersRepository.js'
import { Security } from '../server/shared/security/Security.js'
import { spawnSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'

const args = Object.fromEntries(
  process.argv.slice(2).map(arg => {
    const [key, ...rest] = arg.replace(/^--/, '').split('=')
    return [key, rest.join('=') || 'true']
  })
)

const records = readBootstrapRecords(args)

if (records.length === 0) {
  console.error([
    'Usage:',
    '  bun scripts/create-admin.mjs --email=admin@example.com --domain=example.com',
    '  bun scripts/create-admin.mjs --domains-json=\'[{"domain":"example.com","adminEmail":"admin@example.com"}]\'',
    '',
    'Environment:',
    '  CADUCEUS_BOOTSTRAP_DOMAINS=[{"domain":"example.com","adminEmail":"admin@example.com"}]',
  ].join('\n'))
  process.exit(1)
}

const fylo = await DatabaseRepository.createDb()
const domains = new DomainsRepository(fylo)
const users = new UsersRepository(fylo)
const results = []

for (const record of records) {
  const [, existingDomain] = await domains.findEntry(record.domain)
  if (!existingDomain) {
    await domains.put({
      domain: record.domain,
      inboundEnabled: true,
      routes: [{ id: `store-${record.domain}`, match: `*@${record.domain}`, action: { type: 'store' }, enabled: true }],
    })
  }

  const [, existingUser] = await users.findByEmail(record.email)
  await users.upsertByEmail({
    ...(existingUser || {}),
    email: record.email,
    domains: dedupe([...(existingUser?.domains || []), record.domain]),
    role: 'admin',
  })

  // Identity is the engine's, so it is provisioned by the operator CLI rather
  // than written here. This script owns only the mail-side records beside it.
  const password = randomBytes(24).toString('base64url')
  caduceus(['auth', 'domain', '--domain', record.domain])
  caduceus(['auth', 'mailbox', '--address', record.email])
  caduceus(['auth', 'role', '--address', record.email, '--role', 'domain-admin'])
  const credentials = caduceus([
    'auth', 'credentials', '--address', record.email, '--password', password,
  ])
  results.push({
    email: record.email,
    domain: record.domain,
    role: 'admin',
    password,
    recoveryCodes: credentials.recoveryCodes,
    domainCreated: !existingDomain,
    userCreated: !existingUser,
  })
}

console.log(JSON.stringify(results.length === 1 ? results[0] : { bootstrapped: results }, null, 2))
await fylo.close()

/**
 * Run one `caduceus auth` command and decode its JSON result.
 *
 * The engine's FYLO root has one writer, so provisioning shells out to the
 * operator binary instead of opening a second one from this process.
 *
 * @param {string[]} argv
 * @returns {any}
 */
function caduceus(argv) {
  const binary = process.env.CADUCEUS_BINARY ?? 'caduceus'
  const result = spawnSync(binary, argv, { encoding: 'utf8' })
  if (result.status !== 0) {
    console.error(result.stderr || `caduceus ${argv.join(' ')} failed`)
    process.exit(1)
  }
  try {
    return JSON.parse(result.stdout)
  } catch {
    return {}
  }
}

/**
 * @param {Record<string, string>} args
 * @returns {Array<{ email: string, domain: string }>}
 */
function readBootstrapRecords(args) {
  const rawJson = args['domains-json'] ?? args.domainsJson ?? process.env.CADUCEUS_BOOTSTRAP_DOMAINS
  if (rawJson) return parseBootstrapJson(String(rawJson))

  const email = Security.normalizeEmailAddress(String(args.email ?? process.env.CADUCEUS_ADMIN_EMAIL ?? ''))
  const domain = Security.normalizeDomain(String(args.domain ?? process.env.CADUCEUS_ADMIN_DOMAIN ?? ''))
  if (!email || !domain) return []
  return [{ email, domain }]
}

/** @param {string} raw */
function parseBootstrapJson(raw) {
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch (error) {
    throwUsage(`CADUCEUS_BOOTSTRAP_DOMAINS must be valid JSON: ${error instanceof Error ? error.message : String(error)}`)
  }

  if (!Array.isArray(parsed)) {
    throwUsage('CADUCEUS_BOOTSTRAP_DOMAINS must be a JSON array')
  }

  return parsed.map((item, index) => {
    if (!item || typeof item !== 'object') throwUsage(`Bootstrap record ${index + 1} must be an object`)
    const email = Security.normalizeEmailAddress(String(item.adminEmail ?? item.email ?? ''))
    const domain = Security.normalizeDomain(String(item.domain ?? ''))
    if (!email) throwUsage(`Bootstrap record ${index + 1} has invalid adminEmail`)
    if (!domain) throwUsage(`Bootstrap record ${index + 1} has invalid domain`)
    return { email, domain }
  })
}

/** @param {string} message */
function throwUsage(message) {
  console.error(message)
  process.exit(1)
}

/** @param {string[]} values @returns {string[]} */
function dedupe(values) {
  return [...new Set(values)]
}
