import { createHash, randomUUID } from 'node:crypto'

const MAX_SOURCE_BYTES = 128 * 1024
const MAX_CONTRIBUTIONS = 12
const PACKAGE_ID = /^[a-z][a-z0-9]*(?:\.[a-z0-9][a-z0-9-]*){1,5}$/
const VERSION = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/
const CONTRIBUTION_ID = /^[a-z][a-z0-9-]{0,39}$/
const HEX_COLOR = /^#[0-9a-fA-F]{3,8}$/

export const EXTENSION_PERMISSIONS = Object.freeze([
  'compose:read',
  'compose:write',
  'mail:metadata',
  'ui:notify',
])

export const EXTENSION_SLOTS = Object.freeze([
  'app-toolbar',
  'composer-toolbar',
  'message-toolbar',
])

export const THEME_TOKENS = Object.freeze([
  'danger',
  'on-primary',
  'primary',
  'primary-container',
  'surface',
  'surface-container',
  'surface-container-high',
  'text',
  'text-subtle',
])

/** Parse and normalize one immutable sideload package without persisting trust. */
export function inspectExtensionBundle(bundle) {
  if (!plainObject(bundle) || unknownKeys(bundle, ['manifest', 'source'])) {
    return new Error('Extension package must contain only manifest and source')
  }
  const manifest = normalizeManifest(bundle.manifest)
  if (manifest instanceof Error) return manifest
  const source = typeof bundle.source === 'string' ? bundle.source : ''
  if (Buffer.byteLength(source, 'utf8') > MAX_SOURCE_BYTES) return new Error('Extension source exceeds 128 KiB')
  if (manifest.kind === 'plugin' && source.trim() === '') return new Error('Plugin source is required')
  if (manifest.kind === 'theme' && source !== '') return new Error('Themes cannot contain executable source')
  const normalized = { manifest, source }
  return { ...normalized, digest: sha256(stableJSON(normalized)) }
}

/** Build the owner-scoped durable record after the exact digest was trusted. */
export function trustedExtensionRecord(owner, inspected, now = new Date()) {
  const timestamp = now.toISOString()
  const normalizedOwner = String(owner).toLowerCase()
  return {
    id: `extension-${randomUUID()}`,
    extensionId: inspected.manifest.id,
    owner: normalizedOwner,
    domain: normalizedOwner.split('@')[1] ?? '',
    kind: inspected.manifest.kind,
    displayName: inspected.manifest.displayName,
    version: inspected.manifest.version,
    digest: inspected.digest,
    manifestJson: JSON.stringify(inspected.manifest),
    source: inspected.source,
    enabled: false,
    trustedAt: timestamp,
    installedAt: timestamp,
    updatedAt: timestamp,
  }
}

/** Public settings projection. Source and owner never appear in collection responses. */
export function presentExtension(record, { runtime = false } = {}) {
  const manifest = record.manifest ?? parseManifest(record.manifestJson)
  const result = {
    id: record.extensionId ?? manifest.id,
    displayName: record.displayName ?? manifest.displayName,
    version: record.version ?? manifest.version,
    kind: record.kind ?? manifest.kind,
    digest: record.digest,
    enabled: record.enabled === true,
    trustedAt: record.trustedAt,
    installedAt: record.installedAt,
    updatedAt: record.updatedAt,
    manifest,
  }
  return runtime ? { ...result, source: record.source ?? '' } : result
}

function normalizeManifest(value) {
  const allowed = ['schemaVersion', 'id', 'displayName', 'description', 'version', 'kind', 'engine', 'permissions', 'contributions', 'theme']
  if (!plainObject(value) || unknownKeys(value, allowed)) return new Error('Extension manifest has unknown or invalid fields')
  if (value.schemaVersion !== 1) return new Error('Extension manifest schemaVersion must be 1')
  if (typeof value.id !== 'string' || !PACKAGE_ID.test(value.id) || value.id.length > 128) return new Error('Extension id is invalid')
  if (!boundedText(value.displayName, 80, true) || !boundedText(value.description ?? '', 500, false)) return new Error('Extension display metadata is invalid')
  if (typeof value.version !== 'string' || !VERSION.test(value.version)) return new Error('Extension version must be semantic')
  if (value.kind !== 'plugin' && value.kind !== 'theme') return new Error('Extension kind is invalid')
  if (!plainObject(value.engine) || unknownKeys(value.engine, ['caduceus']) || value.engine.caduceus !== '26') return new Error('Extension engine must target CADUCEUS 26')
  if (!Array.isArray(value.permissions) || value.permissions.length > EXTENSION_PERMISSIONS.length || new Set(value.permissions).size !== value.permissions.length || value.permissions.some(permission => !EXTENSION_PERMISSIONS.includes(permission))) {
    return new Error('Extension permissions are invalid')
  }
  const permissions = [...value.permissions].sort()
  if (value.kind === 'theme') return normalizeThemeManifest(value, permissions)
  return normalizePluginManifest(value, permissions)
}

function normalizePluginManifest(value, permissions) {
  if (value.theme !== undefined) return new Error('Plugins cannot declare theme tokens')
  if (!Array.isArray(value.contributions) || value.contributions.length === 0 || value.contributions.length > MAX_CONTRIBUTIONS) return new Error('Plugin contributions are required and bounded')
  const seen = new Set()
  const contributions = []
  for (const contribution of value.contributions) {
    if (!plainObject(contribution) || unknownKeys(contribution, ['id', 'label', 'slot']) || typeof contribution.id !== 'string' || !CONTRIBUTION_ID.test(contribution.id) || seen.has(contribution.id) || !boundedText(contribution.label, 60, true) || !EXTENSION_SLOTS.includes(contribution.slot)) {
      return new Error('Plugin contribution is invalid')
    }
    seen.add(contribution.id)
    contributions.push({ id: contribution.id, label: contribution.label.trim(), slot: contribution.slot })
  }
  return {
    schemaVersion: 1,
    id: value.id,
    displayName: value.displayName.trim(),
    description: String(value.description ?? '').trim(),
    version: value.version,
    kind: 'plugin',
    engine: { caduceus: '26' },
    permissions,
    contributions,
  }
}

function normalizeThemeManifest(value, permissions) {
  if (permissions.length > 0 || value.contributions !== undefined) return new Error('Themes cannot request permissions or commands')
  const theme = value.theme
  if (!plainObject(theme) || unknownKeys(theme, ['mode', 'tokens']) || !['light', 'dark', 'both'].includes(theme.mode) || !plainObject(theme.tokens)) return new Error('Theme definition is invalid')
  const keys = Object.keys(theme.tokens)
  if (keys.length < 3 || keys.length > THEME_TOKENS.length || keys.some(key => !THEME_TOKENS.includes(key) || typeof theme.tokens[key] !== 'string' || !HEX_COLOR.test(theme.tokens[key]))) {
    return new Error('Theme tokens must be allowlisted hexadecimal colors')
  }
  const tokens = Object.fromEntries(keys.sort().map(key => [key, theme.tokens[key].toLowerCase()]))
  return {
    schemaVersion: 1,
    id: value.id,
    displayName: value.displayName.trim(),
    description: String(value.description ?? '').trim(),
    version: value.version,
    kind: 'theme',
    engine: { caduceus: '26' },
    permissions: [],
    theme: { mode: theme.mode, tokens },
  }
}

function parseManifest(value) {
  try { return JSON.parse(value) } catch { throw new Error('Stored extension manifest is invalid') }
}

function stableJSON(value) {
  if (Array.isArray(value)) return `[${value.map(stableJSON).join(',')}]`
  if (plainObject(value)) return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${stableJSON(value[key])}`).join(',')}}`
  return JSON.stringify(value)
}

function sha256(value) { return createHash('sha256').update(value, 'utf8').digest('hex') }
function plainObject(value) { return !!value && typeof value === 'object' && !Array.isArray(value) && Object.getPrototypeOf(value) === Object.prototype }
function unknownKeys(value, allowed) { return Object.keys(value).some(key => !allowed.includes(key)) }
function boundedText(value, maximum, required) { return typeof value === 'string' && value.length <= maximum && (!required || value.trim() !== '') && !/[\0\r\n]/.test(value) }
