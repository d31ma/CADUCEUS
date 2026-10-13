import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const projectRoot = resolve(import.meta.dirname, '..')
const ledgerPath = resolve(projectRoot, 'docs/mailspring-parity.json')
const requireComplete = process.argv.includes('--require-complete')
const allowedStatuses = new Set(['implemented', 'partial', 'missing'])

const expectedPackages = [
  'account-sidebar',
  'activity',
  'attachments',
  'category-mapper',
  'category-picker',
  'composer',
  'composer-grammar-check',
  'composer-signature',
  'composer-templates',
  'contacts',
  'custom-fonts',
  'custom-sounds',
  'draft-list',
  'events',
  'github-contact-card',
  'link-tracking',
  'list-unsubscribe',
  'main-calendar',
  'mcp-server',
  'message-autoload-images',
  'message-list',
  'message-view-on-github',
  'mode-switch',
  'notifications',
  'onboarding',
  'open-tracking',
  'participant-profile',
  'personal-level-indicators',
  'phishing-detection',
  'preferences',
  'print',
  'remove-tracking-pixels',
  'screenshot-mode',
  'send-and-archive',
  'send-later',
  'send-reminders',
  'system-tray',
  'theme-picker',
  'thread-list',
  'thread-search',
  'thread-sharing',
  'thread-snooze',
  'translation',
  'ui-automatic',
  'ui-dark',
  'ui-darkside',
  'ui-less-is-more',
  'ui-light',
  'ui-taiga',
  'ui-ubuntu',
  'undo-redo',
  'unread-notifications',
]

function fail(message) {
  console.error(`Mailspring parity audit failed: ${message}`)
  process.exitCode = 1
}

if (!existsSync(ledgerPath)) {
  fail('docs/mailspring-parity.json is missing')
  process.exit()
}

/** @type {any} */
let ledger
try {
  ledger = JSON.parse(readFileSync(ledgerPath, 'utf8'))
} catch (error) {
  fail(`ledger is not valid JSON: ${error instanceof Error ? error.message : String(error)}`)
  process.exit()
}

if (ledger.schemaVersion !== 1) fail('schemaVersion must be 1')
if (ledger.upstream?.repository !== 'https://github.com/Foundry376/Mailspring') {
  fail('upstream repository must identify the authoritative Mailspring repository')
}
if (!/^[0-9a-f]{40}$/.test(ledger.upstream?.commit ?? '')) {
  fail('upstream commit must be a full lowercase Git commit')
}

const packages = Array.isArray(ledger.packages) ? ledger.packages : []
const packageIds = packages.map((item) => item.id)
if (JSON.stringify(packageIds) !== JSON.stringify(expectedPackages)) {
  fail(`packages must contain the ${expectedPackages.length} upstream packages in sorted order`)
}

const requirements = [...packages, ...(Array.isArray(ledger.platformCapabilities)
  ? ledger.platformCapabilities
  : [])]
const seen = new Set()
for (const requirement of requirements) {
  if (!requirement || typeof requirement !== 'object') {
    fail('every requirement must be an object')
    continue
  }
  if (typeof requirement.id !== 'string' || !requirement.id) {
    fail('every requirement must have an id')
    continue
  }
  if (seen.has(requirement.id)) fail(`duplicate requirement id ${requirement.id}`)
  seen.add(requirement.id)
  if (typeof requirement.title !== 'string' || !requirement.title) {
    fail(`${requirement.id} must have a title`)
  }
  if (!allowedStatuses.has(requirement.status)) {
    fail(`${requirement.id} has invalid status ${JSON.stringify(requirement.status)}`)
  }
  if (!Array.isArray(requirement.upstreamEvidence) || requirement.upstreamEvidence.length === 0) {
    fail(`${requirement.id} must cite upstream evidence`)
  }
  if (!Array.isArray(requirement.caduceusEvidence)) {
    fail(`${requirement.id} must have a caduceusEvidence array`)
    continue
  }
  if (requirement.status !== 'missing' && requirement.caduceusEvidence.length === 0) {
    fail(`${requirement.id} is ${requirement.status} but has no CADUCEUS evidence`)
  }
  if (requirement.status !== 'implemented' &&
      (typeof requirement.gap !== 'string' || !requirement.gap.trim())) {
    fail(`${requirement.id} is ${requirement.status} but has no explicit gap`)
  }
  for (const relativePath of requirement.caduceusEvidence) {
    if (typeof relativePath !== 'string' || relativePath.startsWith('/') || relativePath.includes('..')) {
      fail(`${requirement.id} has unsafe evidence path ${JSON.stringify(relativePath)}`)
      continue
    }
    if (!existsSync(resolve(projectRoot, relativePath))) {
      fail(`${requirement.id} cites missing evidence ${relativePath}`)
    }
  }
  if (requirement.status === 'implemented' &&
      !requirement.caduceusEvidence.some((entry) => entry.startsWith('tests/'))) {
    fail(`${requirement.id} is implemented but cites no automated test evidence`)
  }
}

const counts = Object.fromEntries([...allowedStatuses].map((status) => [
  status,
  requirements.filter((requirement) => requirement.status === status).length,
]))

if (requireComplete && counts.partial + counts.missing > 0) {
  fail(`completion gate has ${counts.partial} partial and ${counts.missing} missing requirements`)
}

if (!process.exitCode) {
  console.log(
    `Mailspring parity ledger valid: ${requirements.length} requirements ` +
    `(${counts.implemented} implemented, ${counts.partial} partial, ${counts.missing} missing)`,
  )
}
