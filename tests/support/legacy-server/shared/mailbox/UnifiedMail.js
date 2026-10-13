import { randomUUID } from 'node:crypto'
import { ExternalAccounts } from '../accounts/ExternalAccounts.js'
import { AuthBridgeError } from '../auth/AuthBridgeError.js'
import { ExternalMessage } from './ExternalMessage.js'
import { ExternalFolderError } from './ExternalFolderError.js'

export class UnifiedMail {
  static #EXTERNAL_ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/

  static isExternalAccountId(value) {
    return typeof value === 'string' && UnifiedMail.#EXTERNAL_ACCOUNT_ID.test(value)
  }

  /** Read bounded owner-scoped provider rows and map them to the normal mailbox contract. */
  static async externalMailSummaries(token, filter = {}) {
    const response = await ExternalAccounts.unifiedMessages(token, {
      accountId: String(filter.accountId || ''),
      folderRole: String(filter.folderRole || ''),
      folderStableId: String(filter.folderStableId || ''),
      query: String(filter.query || ''),
      ...(typeof filter.read === 'boolean' ? { read: filter.read } : {}),
      ...(typeof filter.starred === 'boolean' ? { starred: filter.starred } : {}),
      ...(typeof filter.receivedAtFrom === 'string' ? { receivedAtFrom: filter.receivedAtFrom } : {}),
      ...(filter.oldestFirst === true ? { oldestFirst: true } : {}),
      limit: Math.max(1, Math.min(1000, Number(filter.limit) || 1000)),
      offset: Math.max(0, Number(filter.offset) || 0),
    })
    return (Array.isArray(response?.messages) ? response.messages : [])
      .map(ExternalMessage.presentExternalSummary)
      .filter(Boolean)
  }

  /**
   * Stable newest-first merge with pagination applied after both mail stores.
   *
   * @param {Array<{ receivedAt?: unknown, id?: unknown }>} hosted
   * @param {Array<{ receivedAt?: unknown, id?: unknown }>} external
   * @param {{ offset?: number, limit?: number }} [pagination]
   */
  static mergeUnifiedMail(hosted, external, { offset = 0, limit } = {}) {
    const merged = [...(hosted || []), ...(external || [])].sort((left, right) => {
      const time = String(right?.receivedAt || '').localeCompare(String(left?.receivedAt || ''))
      return time || String(left?.id || '').localeCompare(String(right?.id || ''))
    })
    const start = Math.max(0, Number(offset) || 0)
    if (limit == null) return merged.slice(start)
    return merged.slice(start, start + Math.max(0, Number(limit) || 0))
  }

  static externalAttachmentSummaries(email) {
    return (email?.attachmentNames || []).map((filename, index) => ({
      id: `extatt.${index}`,
      filename,
      contentType: 'application/octet-stream',
      size: 0,
    }))
  }

  /**
   * Load one external message only through owner-authorized Go operations and
   * enrich its raw index row with account and provider-folder presentation data.
   *
   * @param {string} token
   * @param {string} routeId
   */
  static async loadExternalMessage(token, routeId) {
    const identity = ExternalMessage.parseExternalMessageId(routeId)
    if (!identity) return null
    const [account, folderResponse, source] = await Promise.all([
      ExternalAccounts.get(token, identity.accountId),
      ExternalAccounts.folders(token, identity.accountId),
      ExternalAccounts.messageSource(token, identity.accountId, identity.stableId),
    ])
    if (!account?.id || !source?.message || typeof source.rawRFC822Base64 !== 'string') return null
    const folders = Array.isArray(folderResponse?.folders) ? folderResponse.folders : []
    const folder = folders.find(candidate => candidate?.stableId === source.message.folderStableId)
    return {
      identity,
      account,
      folders,
      indexed: {
        ...source.message,
        accountId: account.id,
        mailboxAddress: account.mailboxAddress,
        accountLabel: account.label,
        accountColor: account.color,
        folderName: folder?.name || source.message.folderStableId,
        folderRole: folder?.role || UnifiedMail.#inferFolderRole(folder?.name || source.message.folderStableId),
      },
      rawRFC822Base64: source.rawRFC822Base64,
    }
  }

  /** Metadata-only variant for bounded bulk mutations and journal recovery. */
  static async loadExternalMessageMetadata(token, routeId) {
    const identity = ExternalMessage.parseExternalMessageId(routeId)
    if (!identity) return null
    const [account, folderResponse, response] = await Promise.all([
      ExternalAccounts.get(token, identity.accountId),
      ExternalAccounts.folders(token, identity.accountId),
      ExternalAccounts.message(token, identity.accountId, identity.stableId),
    ])
    if (!account?.id || !response?.message) return null
    const folders = Array.isArray(folderResponse?.folders) ? folderResponse.folders : []
    const folder = folders.find(candidate => candidate?.stableId === response.message.folderStableId)
    return {
      identity,
      account,
      folders,
      indexed: {
        ...response.message,
        accountId: account.id,
        mailboxAddress: account.mailboxAddress,
        accountLabel: account.label,
        accountColor: account.color,
        folderName: folder?.name || response.message.folderStableId,
        folderRole: folder?.role || UnifiedMail.#inferFolderRole(folder?.name || response.message.folderStableId),
      },
    }
  }

  /** Resolve a normal mailbox folder name to one exact provider folder. */
  static resolveExternalFolder(folders, requested) {
    const wanted = String(requested || '').trim().toLowerCase()
    if (!wanted || !Array.isArray(folders)) return null
    return folders.find(folder => String(folder?.role || '').trim().toLowerCase() === wanted)
      || folders.find(folder => [folder?.stableId, folder?.name]
        .some(value => String(value || '').trim().toLowerCase() === wanted))
      || null
  }

  /**
   * Persist every requested provider mutation before the caller projects it in
   * the UI. IMAP does not provide a multi-property atomic primitive, so each
   * operation has its own durable idempotency identity and reconciliation state.
   *
   * @param {string} token
   * @param {Awaited<ReturnType<typeof loadExternalMessage>>} loaded
   * @param {Record<string, unknown>} patch
   */
  /** @param {string} token @param {any} loaded @param {Record<string, any>} patch @param {{idempotencyPrefix?:string}} [options] */
  static async queueExternalMessagePatch(token, loaded, patch, options = {}) {
    if (!loaded) return []
    const requests = []
    if (typeof patch.read === 'boolean') requests.push({ kind: 'set_read', value: patch.read })
    if (typeof patch.starred === 'boolean') requests.push({ kind: 'set_starred', value: patch.starred })
    if (typeof patch.folder === 'string') {
      const target = UnifiedMail.resolveExternalFolder(loaded.folders, patch.folder)
      if (!target) throw new ExternalFolderError(patch.folder)
      requests.push({ kind: 'move', value: false, targetFolderId: target.stableId, target })
    }
    const mutations = []
    for (const request of requests) {
      mutations.push(await UnifiedMail.#enqueueExternalMutation(token, loaded, {
        idempotencyKey: UnifiedMail.#mutationIdempotencyKey(options.idempotencyPrefix, request.kind),
        kind: request.kind,
        messageStableIds: [loaded.identity.stableId],
        targetFolderId: request.targetFolderId || '',
        value: request.value,
      }))
    }
    return mutations
  }

  static #mutationIdempotencyKey(prefix, kind) {
    const stable = typeof prefix === 'string' ? prefix.trim() : ''
    return stable ? `${stable}-${kind}`.slice(0, 128) : `mailbox-${kind}-${randomUUID()}`
  }

  /** Queue a provider-side permanent delete for one exact owned message. */
  static async queueExternalMessageDelete(token, loaded) {
    if (!loaded) return null
    return UnifiedMail.#enqueueExternalMutation(token, loaded, {
      idempotencyKey: `mailbox-delete-${randomUUID()}`,
      kind: 'delete',
      messageStableIds: [loaded.identity.stableId],
      targetFolderId: '',
      value: false,
    })
  }

  /**
   * A background provider sync may legitimately advance the public account
   * revision after the message was authorized. Refresh once on that exact
   * optimistic-concurrency conflict; all other failures remain fail-closed.
   */
  static async #enqueueExternalMutation(token, loaded, request) {
    let revision = loaded.account.revision
    for (let attempt = 0; attempt < 5; attempt++) {
      try {
        return await ExternalAccounts.mutate(token, loaded.account.id, { ...request, revision })
      } catch (error) {
        if (!(error instanceof AuthBridgeError) || error.status !== 409 || attempt === 4) throw error
      }
      const refreshed = await ExternalAccounts.get(token, loaded.account.id)
      revision = refreshed.revision
    }
    throw new AuthBridgeError('ECONFLICT', 'external account changed; refresh and retry')
  }

  /** Apply a pending provider patch to the already-authorized reader projection. */
  static projectExternalPatch(loaded, patch) {
    if (!loaded) return null
    const indexed = { ...loaded.indexed, flags: [...(loaded.indexed.flags || [])] }
    if (typeof patch.read === 'boolean') indexed.flags = UnifiedMail.#projectFlag(indexed.flags, '\\Seen', patch.read)
    if (typeof patch.starred === 'boolean') indexed.flags = UnifiedMail.#projectFlag(indexed.flags, '\\Flagged', patch.starred)
    if (typeof patch.folder === 'string') {
      const target = UnifiedMail.resolveExternalFolder(loaded.folders, patch.folder)
      if (target) {
        indexed.folderStableId = target.stableId
        indexed.folderName = target.name
        indexed.folderRole = target.role || UnifiedMail.#inferFolderRole(target.name)
      }
    }
    return { ...loaded, indexed }
  }

  static #inferFolderRole(value) {
    const folder = String(value || '').trim().toLowerCase()
    return ['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'].includes(folder) ? folder : ''
  }

  static #projectFlag(flags, target, enabled) {
    const values = flags.filter(flag => String(flag).toLowerCase() !== target.toLowerCase())
    if (enabled) values.push(target)
    return values
  }
}
