import { createHash, randomUUID } from 'node:crypto'

export class Extensions {
  static #MAX_SOURCE_BYTES = 128 * 1024

  static #MAX_CONTRIBUTIONS = 12

  static #PACKAGE_ID = /^[a-z][a-z0-9]*(?:\.[a-z0-9][a-z0-9-]*){1,5}$/

  static #VERSION = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/

  static #CONTRIBUTION_ID = /^[a-z][a-z0-9-]{0,39}$/

  static #HEX_COLOR = /^#[0-9a-fA-F]{3,8}$/

  static EXTENSION_PERMISSIONS = Object.freeze([
    'compose:read',
    'compose:write',
    'mail:metadata',
    'ui:notify',
  ])

  static EXTENSION_SLOTS = Object.freeze([
    'app-toolbar',
    'composer-toolbar',
    'message-toolbar',
  ])

  static THEME_TOKENS = Object.freeze([
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
  static inspectExtensionBundle(bundle) {
    if (!Extensions.#plainObject(bundle) || Extensions.#unknownKeys(bundle, ['manifest', 'source'])) {
      return new Error('Extension package must contain only manifest and source')
    }
    const manifest = Extensions.#normalizeManifest(bundle.manifest)
    if (manifest instanceof Error) return manifest
    const source = typeof bundle.source === 'string' ? bundle.source : ''
    if (Buffer.byteLength(source, 'utf8') > Extensions.#MAX_SOURCE_BYTES) return new Error('Extension source exceeds 128 KiB')
    if (manifest.kind === 'plugin' && source.trim() === '') return new Error('Plugin source is required')
    if (manifest.kind === 'theme' && source !== '') return new Error('Themes cannot contain executable source')
    const normalized = { manifest, source }
    return { ...normalized, digest: Extensions.#sha256(Extensions.#stableJSON(normalized)) }
  }

  /** Build the owner-scoped durable record after the exact digest was trusted. */
  static trustedExtensionRecord(owner, inspected, now = new Date()) {
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
  static presentExtension(record, { runtime = false } = {}) {
    const manifest = record.manifest ?? Extensions.#parseManifest(record.manifestJson)
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

  static #normalizeManifest(value) {
    const allowed = ['schemaVersion', 'id', 'displayName', 'description', 'version', 'kind', 'engine', 'permissions', 'contributions', 'theme']
    if (!Extensions.#plainObject(value) || Extensions.#unknownKeys(value, allowed)) return new Error('Extension manifest has unknown or invalid fields')
    if (value.schemaVersion !== 1) return new Error('Extension manifest schemaVersion must be 1')
    if (typeof value.id !== 'string' || !Extensions.#PACKAGE_ID.test(value.id) || value.id.length > 128) return new Error('Extension id is invalid')
    if (!Extensions.#boundedText(value.displayName, 80, true) || !Extensions.#boundedText(value.description ?? '', 500, false)) return new Error('Extension display metadata is invalid')
    if (typeof value.version !== 'string' || !Extensions.#VERSION.test(value.version)) return new Error('Extension version must be semantic')
    if (value.kind !== 'plugin' && value.kind !== 'theme') return new Error('Extension kind is invalid')
    if (!Extensions.#plainObject(value.engine) || Extensions.#unknownKeys(value.engine, ['caduceus']) || value.engine.caduceus !== '26') return new Error('Extension engine must target CADUCEUS 26')
    if (!Array.isArray(value.permissions) || value.permissions.length > Extensions.EXTENSION_PERMISSIONS.length || new Set(value.permissions).size !== value.permissions.length || value.permissions.some(permission => !Extensions.EXTENSION_PERMISSIONS.includes(permission))) {
      return new Error('Extension permissions are invalid')
    }
    const permissions = [...value.permissions].sort()
    if (value.kind === 'theme') return Extensions.#normalizeThemeManifest(value, permissions)
    return Extensions.#normalizePluginManifest(value, permissions)
  }

  static #normalizePluginManifest(value, permissions) {
    if (value.theme !== undefined) return new Error('Plugins cannot declare theme tokens')
    if (!Array.isArray(value.contributions) || value.contributions.length === 0 || value.contributions.length > Extensions.#MAX_CONTRIBUTIONS) return new Error('Plugin contributions are required and bounded')
    const seen = new Set()
    const contributions = []
    for (const contribution of value.contributions) {
      if (!Extensions.#plainObject(contribution) || Extensions.#unknownKeys(contribution, ['id', 'label', 'slot']) || typeof contribution.id !== 'string' || !Extensions.#CONTRIBUTION_ID.test(contribution.id) || seen.has(contribution.id) || !Extensions.#boundedText(contribution.label, 60, true) || !Extensions.EXTENSION_SLOTS.includes(contribution.slot)) {
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

  static #normalizeThemeManifest(value, permissions) {
    if (permissions.length > 0 || value.contributions !== undefined) return new Error('Themes cannot request permissions or commands')
    const theme = value.theme
    if (!Extensions.#plainObject(theme) || Extensions.#unknownKeys(theme, ['mode', 'tokens']) || !['light', 'dark', 'both'].includes(theme.mode) || !Extensions.#plainObject(theme.tokens)) return new Error('Theme definition is invalid')
    const keys = Object.keys(theme.tokens)
    if (keys.length < 3 || keys.length > Extensions.THEME_TOKENS.length || keys.some(key => !Extensions.THEME_TOKENS.includes(key) || typeof theme.tokens[key] !== 'string' || !Extensions.#HEX_COLOR.test(theme.tokens[key]))) {
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

  static #parseManifest(value) {
    try { return JSON.parse(value) } catch { throw new Error('Stored extension manifest is invalid') }
  }

  static #stableJSON(value) {
    if (Array.isArray(value)) return `[${value.map(Extensions.#stableJSON).join(',')}]`
    if (Extensions.#plainObject(value)) return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${Extensions.#stableJSON(value[key])}`).join(',')}}`
    return JSON.stringify(value)
  }

  static #sha256(value) { return createHash('sha256').update(value, 'utf8').digest('hex') }

  static #plainObject(value) { return !!value && typeof value === 'object' && !Array.isArray(value) && Object.getPrototypeOf(value) === Object.prototype }

  static #unknownKeys(value, allowed) { return Object.keys(value).some(key => !allowed.includes(key)) }

  static #boundedText(value, maximum, required) { return typeof value === 'string' && value.length <= maximum && (!required || value.trim() !== '') && !/[\0\r\n]/.test(value) }
}
