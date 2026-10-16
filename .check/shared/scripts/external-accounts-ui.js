// @ts-check

const PROVIDERS = new Set(['gmail', 'microsoft', 'imap'])
const AUTHENTICATION = new Set(['oauth', 'password'])
const STATES = new Set(['ready', 'syncing', 'idle', 'auth_required', 'degraded', 'paused', 'deleting'])
const SECURITY = new Set(['tls', 'starttls', 'none'])
const ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/

/** @param {unknown} value */
function record(value) { return value && typeof value === 'object' && !Array.isArray(value) ? /** @type {Record<string, any>} */ (value) : {} }

/** @param {unknown} value */
function text(value) { return typeof value === 'string' ? value.trim() : '' }

/** @param {unknown} value */
function port(value) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 && parsed <= 65535 ? String(parsed) : ''
}

/** @param {unknown} value */
function endpoint(value) {
  const source = record(value)
  const security = text(source.security)
  const host = text(source.host).toLowerCase()
  const normalizedPort = port(source.port)
  if (!host || !normalizedPort || !SECURITY.has(security)) return null
  return {
    host, port: Number(normalizedPort), security,
    username: text(source.username), allowInsecureTLS: source.allowInsecureTLS === true,
  }
}

/** Keep only catalog fields the UI is prepared to render. */
export function normalizeProviderCatalog(value) {
  if (!Array.isArray(value)) return []
  return value.flatMap(candidate => {
    const source = record(candidate)
    const id = text(source.id)
    const name = text(source.name)
    const provider = text(source.provider)
    const authentication = text(source.authentication)
    const guidance = text(source.guidance) || 'none'
    if (!id || !name || !PROVIDERS.has(provider) || !AUTHENTICATION.has(authentication)) return []
    const presetSource = record(source.preset)
    const imap = endpoint(presetSource.imap)
    const smtp = endpoint(presetSource.smtp)
    const preset = imap && smtp ? { imap, smtp, containerFolder: text(presetSource.containerFolder) } : null
    return [{ id, name, provider, authentication, guidance, configured: source.configured === true, preset }]
  })
}

/** Convert discovery output to non-secret native form values. */
export function connectionDraft(value, mailboxAddress = '') {
  const source = record(value)
  const connection = record(source.connection)
  const imap = endpoint(connection.imap)
  const smtp = endpoint(connection.smtp)
  const cardDAV = record(connection.carddav)
  const calDAV = record(connection.caldav)
  const address = text(mailboxAddress).toLowerCase()
  return {
    choiceId: text(source.choiceId) || 'imap', source: text(source.source) || 'manual', mailboxAddress: address,
    label: text(source.label) || address,
    username: imap?.username || smtp?.username || address,
    imapHost: imap?.host || '', imapPort: imap ? String(imap.port) : '993', imapSecurity: imap?.security || 'tls',
    smtpHost: smtp?.host || '', smtpPort: smtp ? String(smtp.port) : '587', smtpSecurity: smtp?.security || 'starttls',
    cardDAVURL: text(cardDAV.url), cardDAVUsername: text(cardDAV.username),
    calDAVURL: text(calDAV.url), calDAVUsername: text(calDAV.username),
    containerFolder: text(connection.containerFolder),
  }
}

/** Keep a redacted, bounded list; unknown fields (including credentials) are dropped. */
export function normalizeExternalAccounts(value) {
  if (!Array.isArray(value)) return []
  return value.slice(0, 50).flatMap(candidate => {
    const source = record(candidate)
    const id = text(source.id)
    const mailboxAddress = text(source.mailboxAddress)
    const provider = text(source.provider)
    const state = text(source.state)
    const revision = Number(source.revision)
    if (!ACCOUNT_ID.test(id) || !mailboxAddress || !PROVIDERS.has(provider) || !STATES.has(state) || !Number.isInteger(revision) || revision < 1) return []
    const progressSource = record(source.progress)
    const failureSource = record(source.failure)
    const completed = Number(progressSource.completed)
    const total = progressSource.total == null ? null : Number(progressSource.total)
    return [{
      id, mailboxAddress, provider, state, revision,
      authIdentity: text(source.authIdentity), sharedMailboxAddress: text(source.sharedMailboxAddress),
      label: text(source.label) || mailboxAddress, color: /^#[0-9A-Fa-f]{6}$/.test(text(source.color)) ? text(source.color) : '#5f6368',
      order: Number.isInteger(Number(source.order)) ? Number(source.order) : 0,
      capabilities: record(source.capabilities), connection: record(source.connection),
      progress: text(progressSource.phase) && Number.isInteger(completed) && completed >= 0
        ? { phase: text(progressSource.phase), completed, total: typeof total === 'number' && Number.isInteger(total) && total >= 0 ? total : null }
        : null,
      failure: text(failureSource.message)
        ? { code: text(failureSource.code), message: text(failureSource.message), retryable: failureSource.retryable === true }
        : null,
      updatedAt: text(source.updatedAt),
    }]
  }).sort((left, right) => left.order - right.order || left.label.localeCompare(right.label))
}

/** Connected accounts that can currently be selected as composer identities. */
export function externalSenderIdentities(value) {
  const seen = new Set()
  return normalizeExternalAccounts(value).flatMap(account => {
    const address = account.mailboxAddress.toLowerCase()
    if (!account.capabilities.send || !['ready', 'syncing', 'idle', 'degraded'].includes(account.state) || seen.has(address)) return []
    seen.add(address)
    return [{ address, label: account.label, accountId: account.id, provider: account.provider, color: account.color }]
  })
}

/** @param {Record<string, any>} account */
export function accountActions(account) {
  const state = text(account?.state)
  return {
    canPause: ['ready', 'syncing', 'idle', 'degraded'].includes(state),
    canResume: state === 'paused',
    canReconnect: ['auth_required', 'degraded', 'paused'].includes(state),
    canRetry: state === 'degraded',
    canRemove: state !== 'deleting',
  }
}

/** @param {Record<string, any>} account */
export function accountProgressLabel(account) {
  const progress = record(account?.progress)
  const state = text(account?.state)
  if (progress.phase) {
    const phase = text(progress.phase).replaceAll('_', ' ')
    const count = progress.total == null ? String(progress.completed ?? 0) : `${progress.completed ?? 0} of ${progress.total}`
    return `Syncing ${phase} · ${count}`
  }
  return state.replaceAll('_', ' ').replace(/^./, character => character.toUpperCase())
}
