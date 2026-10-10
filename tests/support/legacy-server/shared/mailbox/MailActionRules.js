import { createHash, randomUUID } from 'node:crypto'
import { AsyncLocalStorage } from 'node:async_hooks'
import { EmailsRecords } from './EmailsRecords.js'
import { CategoryPath } from './CategoryPath.js'
import { MailActionLabels } from './MailActionLabels.js'
import { ExternalMessage } from './ExternalMessage.js'
import { UnifiedMail } from './UnifiedMail.js'
import { ExternalAccounts } from '../accounts/ExternalAccounts.js'
import { ExternalFolderError } from './ExternalFolderError.js'
import { MailActionError } from '../errors/MailActionError.js'

export class MailActionRules {
  static MUTABLE_FIELDS = Object.freeze(['folder', 'labels', 'read', 'starred'])

  static INCOMPLETE_STATUSES = new Set(['prepared', 'undoing', 'redoing'])

  static ACTION_TTL_MS = 10 * 60 * 1000

  static MAX_ACTION_MESSAGES = 100

  static MAX_ID_LENGTH = 500

  static PROVIDER_MUTATION_TIMEOUT_MS = 60_000

  static PROVIDER_MUTATION_POLL_MS = 100

  static async waitForProviderMutation(token, accountId, mutationId) {
    const deadline = Date.now() + MailActionRules.PROVIDER_MUTATION_TIMEOUT_MS
    while (Date.now() < deadline) {
      const mutation = await ExternalAccounts.mutation(token, accountId, mutationId)
      if (mutation?.state === 'applied') return mutation
      if (mutation?.state === 'failed') {
        throw new MailActionError('The connected account rejected this action. Refresh to review the provider version.', 409)
      }
      await new Promise(resolve => setTimeout(resolve, MailActionRules.PROVIDER_MUTATION_POLL_MS))
    }
    throw new MailActionError('The connected account action is still reconciling. Try again shortly.', 503)
  }

  static parsePatch(input) {
    if (!input || typeof input !== 'object' || Array.isArray(input)) throw new MailActionError('patch is required')
    const source = /** @type {Record<string, unknown>} */ (input)
    const unknown = Object.keys(source).filter(key => !MailActionRules.MUTABLE_FIELDS.includes(key))
    if (unknown.length > 0) throw new MailActionError(`Unsupported patch field: ${unknown[0]}`)
    const patch = {}
    if ('folder' in source) {
      if (typeof source.folder !== 'string' || !source.folder.trim()) throw new MailActionError('folder must be a non-empty string')
      const folder = CategoryPath.normalizeCategoryPath(source.folder)
      if (!folder) throw new MailActionError('folder must be a valid category path')
      patch.folder = folder
    }
    if ('labels' in source) {
      const result = MailActionLabels.parseLabelOperation(source.labels)
      if (!result.ok) throw new MailActionError(result.error)
      patch.labels = result.operation
    }
    for (const field of ['read', 'starred']) {
      if (!(field in source)) continue
      if (typeof source[field] !== 'boolean') throw new MailActionError(`${field} must be a boolean`)
      patch[field] = source[field]
    }
    if (Object.keys(patch).length === 0) throw new MailActionError('No supported fields to update')
    return patch
  }

  static parseChanges(record) {
    let value
    try { value = JSON.parse(record.changesJson) } catch { throw new MailActionError('Mail action record is corrupt', 503) }
    if (!Array.isArray(value) || value.length === 0 || value.length > MailActionRules.MAX_ACTION_MESSAGES) {
      throw new MailActionError('Mail action record is corrupt', 503)
    }
    for (const change of value) {
      if (!change || typeof change !== 'object' || typeof change.id !== 'string' || typeof change.domain !== 'string') {
        throw new MailActionError('Mail action record is corrupt', 503)
      }
      MailActionRules.validateStoredState(change.before)
      MailActionRules.validateStoredState(change.after)
      if (Object.keys(change.before).sort().join() !== Object.keys(change.after).sort().join()) {
        throw new MailActionError('Mail action record is corrupt', 503)
      }
      if (change.source !== undefined && change.source !== 'provider') throw new MailActionError('Mail action record is corrupt', 503)
      if (change.source === 'provider') {
        if (!/^ext_[A-Za-z0-9_-]{20,80}$/.test(change.accountId || '') || typeof change.stableId !== 'string' ||
          change.stableId.length < 1 || change.stableId.length > 512 || /[\0\r\n]/.test(change.stableId)) {
          throw new MailActionError('Mail action record is corrupt', 503)
        }
        for (const target of ['before', 'after']) {
          const ids = change.providerMutations?.[target]
          if (!Array.isArray(ids) || ids.length > MailActionRules.MUTABLE_FIELDS.length || ids.some(id => !/^mut_[A-Za-z0-9_-]{20,80}$/.test(id))) {
            throw new MailActionError('Mail action record is corrupt', 503)
          }
          const generation = change.providerGenerations?.[target]
          if (!Number.isSafeInteger(generation) || generation < 0 || generation > 10_000) {
            throw new MailActionError('Mail action record is corrupt', 503)
          }
        }
      }
    }
    return value
  }

  static validateStoredState(state) {
    if (!state || typeof state !== 'object' || Array.isArray(state)) throw new MailActionError('Mail action record is corrupt', 503)
    const fields = Object.keys(state)
    if (fields.length === 0 || fields.some(field => !MailActionRules.MUTABLE_FIELDS.includes(field))) throw new MailActionError('Mail action record is corrupt', 503)
    if ('folder' in state && (typeof state.folder !== 'string' || !state.folder)) throw new MailActionError('Mail action record is corrupt', 503)
    if ('labels' in state && (!Array.isArray(state.labels) || state.labels.length > 100 || MailActionLabels.normalizeStoredLabels(state.labels).length !== state.labels.length)) {
      throw new MailActionError('Mail action record is corrupt', 503)
    }
    if ('read' in state && typeof state.read !== 'boolean') throw new MailActionError('Mail action record is corrupt', 503)
    if ('starred' in state && typeof state.starred !== 'boolean') throw new MailActionError('Mail action record is corrupt', 503)
  }

  static ensureDomains(changes, domains) {
    if (changes.some(change => change.source !== 'provider' && !domains.includes(change.domain))) {
      throw new MailActionError('Mail action not found', 404)
    }
  }

  static fieldValue(email, field) {
    if (field === 'folder') return email.folder || 'inbox'
    if (field === 'labels') return MailActionLabels.normalizeStoredLabels(email.labels)
    return email[field] === true
  }

  static matchesState(email, state) {
    return Object.entries(state).every(([field, value]) => MailActionRules.equalValue(MailActionRules.fieldValue(email, field), value))
  }

  static sameState(left, right) {
    const keys = Object.keys(left)
    return keys.length === Object.keys(right).length && keys.every(key => MailActionRules.equalValue(left[key], right[key]))
  }

  static equalValue(left, right) {
    if (!Array.isArray(left) || !Array.isArray(right)) return left === right
    return left.length === right.length && left.every((value, index) => value === right[index])
  }

  /**
   * @param {Record<string, string | boolean>} patch
   * @param {number} count
   * @returns {import('../repositories/MailActionsHelper.js').MailActionEntry['kind']}
   */
  static actionKind(patch, count) {
    const fields = Object.keys(patch)
    if (count > 1 || fields.length > 1) return 'bulk'
    if (patch.folder === 'archive') return 'archive'
    if (patch.folder === 'trash') return 'trash'
    if (typeof patch.folder === 'string') return 'move'
    if (patch.read === true) return 'mark-read'
    if (patch.read === false) return 'mark-unread'
    if (patch.starred === true) return 'star'
    if (patch.labels) return 'label'
    return 'unstar'
  }

  static actionResult(actionId, record, status, emails) {
    return {
      actionId,
      status,
      kind: record.kind,
      count: emails.length,
      expiresAt: record.expiresAt,
      emails: emails.map(email => ({
        id: email.id,
        domain: email.domain,
        folder: email.folder || 'inbox',
        labels: MailActionLabels.normalizeStoredLabels(email.labels),
        read: email.read === true,
        starred: email.starred === true,
        ...(email.external ? { external: true, providerAccountId: email.providerAccountId } : {}),
      })),
    }
  }

  /** @param {string[]} ids @param {Record<string, string|boolean>} patch */
  static requestFingerprint(ids, patch) {
    const normalizedPatch = Object.fromEntries(Object.entries(patch).sort(([left], [right]) => left.localeCompare(right)))
    return createHash('sha256').update(JSON.stringify({ ids, patch: normalizedPatch })).digest('hex')
  }

  /** Validate the public bulk-action contract at the service boundary. */
  static parseInput(input) {
    if (!input || typeof input !== 'object' || Array.isArray(input)) {
      throw new MailActionError('Action payload required')
    }
    const body = /** @type {Record<string, unknown>} */ (input)
    if (!Array.isArray(body.ids) || body.ids.length === 0) {
      throw new MailActionError('ids must be a non-empty array')
    }
    if (body.ids.length > MailActionRules.MAX_ACTION_MESSAGES) {
      throw new MailActionError(`ids cannot contain more than ${MailActionRules.MAX_ACTION_MESSAGES} messages`)
    }
    const ids = body.ids.map(value => typeof value === 'string' ? value.trim() : '')
    if (ids.some(id => !id || id.length > MailActionRules.MAX_ID_LENGTH)) {
      throw new MailActionError(`every id must be between 1 and ${MailActionRules.MAX_ID_LENGTH} characters`)
    }
    if (new Set(ids).size !== ids.length) throw new MailActionError('ids must not contain duplicates')
    let clientRequestId
    if (body.clientRequestId !== undefined) {
      if (typeof body.clientRequestId !== 'string' || !/^[A-Fa-f0-9-]{36}$/.test(body.clientRequestId)) {
        throw new MailActionError('clientRequestId must be a UUID')
      }
      clientRequestId = body.clientRequestId.toLowerCase()
    }
    return { ids, patch: MailActionRules.parsePatch(body.patch), ...(clientRequestId ? { clientRequestId } : {}) }
  }
}
