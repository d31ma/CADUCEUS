import { createHash, randomUUID } from 'node:crypto'
import { AsyncLocalStorage } from 'node:async_hooks'
import { canAccessEmail } from '../repositories/EmailsRepository.js'
import { normalizeCategoryPath } from '../../../shared/mailbox/category-path.js'
import { applyLabelOperation, normalizeStoredLabels, parseLabelOperation } from '../../../shared/mailbox/mail-action-labels.js'
import { externalMessageId, parseExternalMessageId, presentExternalSummary } from '../../../shared/mailbox/external-message.js'
import {
  loadExternalMessageMetadata,
  projectExternalPatch,
  queueExternalMessagePatch,
} from '../../../shared/mailbox/unified-mail.js'
import { ExternalAccounts } from '../../../shared/accounts/ExternalAccounts.js'
import { ExternalFolderError } from '../../../shared/mailbox/ExternalFolderError.js'

const MUTABLE_FIELDS = Object.freeze(['folder', 'labels', 'read', 'starred'])
const INCOMPLETE_STATUSES = new Set(['prepared', 'undoing', 'redoing'])
const ACTION_TTL_MS = 10 * 60 * 1000
const MAX_ACTION_MESSAGES = 100
const MAX_ID_LENGTH = 500
const PROVIDER_MUTATION_TIMEOUT_MS = 60_000
const PROVIDER_MUTATION_POLL_MS = 100

export class MailActionError extends Error {
  /** @param {string} message @param {number} [status] */
  constructor(message, status = 400) {
    super(message)
    this.status = status
  }
}

export class MailActionService {
  /**
   * @param {import('../repositories/MailActionsRepository.js').MailActionsRepository} actions
   * @param {import('../repositories/EmailsRepository.js').EmailsRepository} emails
   * @param {import('./CategoryMappingService.js').CategoryMappingService} categoryMappings
   */
  constructor(actions, emails, categoryMappings) {
    this.actions = actions
    this.emails = emails
    this.categoryMappings = categoryMappings
    this._tails = new Map()
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
    if (body.ids.length > MAX_ACTION_MESSAGES) {
      throw new MailActionError(`ids cannot contain more than ${MAX_ACTION_MESSAGES} messages`)
    }
    const ids = body.ids.map(value => typeof value === 'string' ? value.trim() : '')
    if (ids.some(id => !id || id.length > MAX_ID_LENGTH)) {
      throw new MailActionError(`every id must be between 1 and ${MAX_ID_LENGTH} characters`)
    }
    if (new Set(ids).size !== ids.length) throw new MailActionError('ids must not contain duplicates')
    let clientRequestId
    if (body.clientRequestId !== undefined) {
      if (typeof body.clientRequestId !== 'string' || !/^[A-Fa-f0-9-]{36}$/.test(body.clientRequestId)) {
        throw new MailActionError('clientRequestId must be a UUID')
      }
      clientRequestId = body.clientRequestId.toLowerCase()
    }
    return { ids, patch: parsePatch(body.patch), ...(clientRequestId ? { clientRequestId } : {}) }
  }

  /** @param {{ids:string[],patch:Record<string, any>,clientRequestId?:string,ownerEmail:string,domains:string[],providerToken?:string,now?:number}} input */
  async apply(input) {
    return await this._serial(input.ownerEmail, async () => {
      await this._reconcileOwner(input.ownerEmail, input.domains, input.providerToken)
      const now = input.now ?? Date.now()
      const fingerprint = requestFingerprint(input.ids, input.patch)
      if (input.clientRequestId) {
        const [existingId, existing] = await this.actions.findByClientRequestId(input.ownerEmail, input.clientRequestId)
        if (existingId && existing) {
          if (existing.requestFingerprint !== fingerprint) {
            throw new MailActionError('clientRequestId was already used for another action', 409)
          }
          const changes = parseChanges(existing)
          ensureDomains(changes, input.domains)
          const messages = await this._authorizeMessages(changes.map(change => change.id), input.ownerEmail, input.domains, input.providerToken)
          const target = existing.status === 'undone' ? 'before' : 'after'
          if (!await this._statesAccepted(messages, changes, target, input.providerToken)) {
            throw new MailActionError('Messages changed after this action; no changes were overwritten', 409)
          }
          return actionResult(existingId, existing, existing.status, messages)
        }
      }
      const messages = await this._authorizeMessages(input.ids, input.ownerEmail, input.domains, input.providerToken)
      const changes = await this._buildChanges(messages, input.patch, input.ownerEmail)
      if (changes.length === 0) throw new MailActionError('Messages already match this action', 409)

      /** @type {import('../repositories/MailActionsRepository.js').MailActionEntry} */
      const record = {
        id: `mail-action-${randomUUID()}`,
        ownerEmail: input.ownerEmail.toLowerCase(),
        // The action journal is owned by the authenticated CADUCEUS mailbox,
        // even when every target lives at a remote provider domain. This keeps
        // FYLO gid ownership on the hosted tenant; remote identities stay in
        // the bounded opaque changes journal and are reauthorized by Go.
        domain: input.ownerEmail.toLowerCase().split('@')[1],
        kind: actionKind(input.patch, changes.length),
        status: /** @type {const} */ ('prepared'),
        changesJson: JSON.stringify(changes),
        createdAt: now,
        updatedAt: now,
        expiresAt: now + ACTION_TTL_MS,
        lastError: '',
        ...(input.clientRequestId ? {
          clientRequestId: input.clientRequestId,
          requestFingerprint: fingerprint,
        } : {}),
      }
      const actionId = await this.actions.put(record)
      if (typeof actionId !== 'string' || !/^[A-Za-z0-9_-]+$/.test(actionId)) {
        throw new MailActionError('FYLO did not return a mail action identifier', 503)
      }
      try {
        const updated = await this._writeTarget(changes, 'after', input.ownerEmail, input.domains, input.providerToken, actionId)
        await this._updateJournal(actionId, {
          status: 'applied', changesJson: JSON.stringify(changes), updatedAt: Date.now(), lastError: '',
        }, input.ownerEmail)
        return actionResult(actionId, record, 'applied', updated)
      } catch (error) {
        const detail = error instanceof Error ? error.message : 'mail action failed'
        const rolledBack = await this._rollbackPrepared(changes, input.ownerEmail, input.domains, input.providerToken, actionId)
        if (rolledBack) {
          await this._updateJournal(actionId, {
            status: 'failed', updatedAt: Date.now(), lastError: detail.slice(0, 200),
          }, input.ownerEmail).catch(() => false)
        }
        throw error
      }
    })
  }

  /** @param {string} id @param {string} ownerEmail @param {string[]} domains @param {number} [now] */
  async undo(id, ownerEmail, domains, now = Date.now(), providerToken = '') {
    return await this._serial(ownerEmail, async () => {
      const records = await this._reconcileOwner(ownerEmail, domains, providerToken)
      return await this._transition(id, ownerEmail, domains, {
        from: 'applied', intermediate: 'undoing', to: 'undone', source: 'after', target: 'before', now,
      }, providerToken, records)
    })
  }

  /** @param {string} id @param {string} ownerEmail @param {string[]} domains @param {number} [now] */
  async redo(id, ownerEmail, domains, now = Date.now(), providerToken = '') {
    return await this._serial(ownerEmail, async () => {
      const records = await this._reconcileOwner(ownerEmail, domains, providerToken)
      return await this._transition(id, ownerEmail, domains, {
        from: 'undone', intermediate: 'redoing', to: 'applied', source: 'before', target: 'after', now,
      }, providerToken, records)
    })
  }

  /** Recover interrupted work visible to this exact mailbox before accepting another mutation. */
  async _reconcileOwner(ownerEmail, domains, providerToken = '') {
    const owner = ownerEmail.toLowerCase()
    const records = await this.actions.listForOwner(owner)
    for (const [actionId, record] of records) {
      if (!INCOMPLETE_STATUSES.has(record.status)) continue
      let changes
      try {
		changes = parseChanges(record)
		ensureDomains(changes, domains)
		const target = record.status === 'undoing' ? 'before' : 'after'
		const source = target === 'before' ? 'after' : 'before'
		await this._refreshProviderIdentities(changes, target, providerToken, actionId, owner)
		const messages = await this._authorizeMessages(changes.map(change => change.id), owner, domains, providerToken)
        if (!await this._statesAccepted(messages, changes, target, providerToken) &&
          !await this._statesAccepted(messages, changes, source, providerToken)) {
          throw new MailActionError('Messages changed while an action was interrupted', 409)
        }
        await this._writeTarget(changes, target, owner, domains, providerToken, actionId)
        const nextStatus = target === 'before' ? 'undone' : 'applied'
        await this.actions.update(actionId, {
          status: nextStatus, changesJson: JSON.stringify(changes), updatedAt: Date.now(), lastError: '',
        })
        record.status = nextStatus
        record.changesJson = JSON.stringify(changes)
      } catch (error) {
        const detail = error instanceof Error ? error.message : 'action recovery failed'
        await this.actions.update(actionId, {
          status: 'failed', updatedAt: Date.now(), lastError: detail.slice(0, 200),
        }).catch(() => false)
        record.status = 'failed'
      }
    }
    return records
  }

  async _transition(id, ownerEmail, domains, transition, providerToken = '', records = []) {
    const preloaded = records.find(([docId, entry]) => docId === id || entry?.id === id)
    const [docId, record] = preloaded || await this.actions.findById(id, ownerEmail)
    if (!docId || !record) throw new MailActionError('Mail action not found', 404)
    if (typeof record.ownerEmail !== 'string' || record.ownerEmail.toLowerCase() !== ownerEmail.toLowerCase()) {
      throw new MailActionError('Mail action not found', 404)
    }
    const changes = parseChanges(record)
    ensureDomains(changes, domains)
    if (record.expiresAt < transition.now) throw new MailActionError('The undo window has expired', 409)
    if (record.status === transition.to) {
      const current = await this._authorizeMessages(changes.map(change => change.id), ownerEmail, domains, providerToken)
      if (await this._statesAccepted(current, changes, transition.target, providerToken)) {
        return actionResult(id, record, transition.to, current)
      }
    }
    if (record.status !== transition.from) {
      throw new MailActionError(`Mail action cannot be changed from ${record.status}`, 409)
    }
    const messages = await this._authorizeMessages(changes.map(change => change.id), ownerEmail, domains, providerToken)
    if (!await this._statesAccepted(messages, changes, transition.source, providerToken)) {
      throw new MailActionError('Messages changed after this action; no changes were overwritten', 409)
    }

    await this.actions.update(docId, { status: transition.intermediate, updatedAt: transition.now })
    const updated = await this._writeTarget(changes, transition.target, ownerEmail, domains, providerToken, id)
    await this.actions.update(docId, {
      status: transition.to, changesJson: JSON.stringify(changes), updatedAt: Date.now(), lastError: '',
    })
    return actionResult(id, record, transition.to, updated)
  }

  async _authorizeMessages(ids, ownerEmail, domains, providerToken = '') {
    const messages = []
    for (const id of ids) {
      if (parseExternalMessageId(id)) {
        if (!providerToken) throw new MailActionError('Connected account authorization is unavailable', 503)
        const loaded = await loadExternalMessageMetadata(providerToken, id)
        const message = loaded ? presentExternalSummary(loaded.indexed) : null
        if (!loaded || !message) throw new MailActionError('Email not found', 404)
        messages.push(message)
        continue
      }
      const [, email] = await this.emails.findById(id)
      if (!email || !domains.includes(email.domain) || !canAccessEmail(email, ownerEmail)) {
        throw new MailActionError('Email not found', 404)
      }
      messages.push(email)
    }
    return messages
  }

  async _buildChanges(messages, patch, ownerEmail) {
    const changes = []
    for (const email of messages) {
      if (email.external && patch.labels) {
        throw new MailActionError('Connected accounts do not expose portable label mutations')
      }
      const before = {}
      const after = {}
      for (const field of MUTABLE_FIELDS) {
        if (!(field in patch)) continue
        before[field] = fieldValue(email, field)
        after[field] = field === 'labels'
          ? applyLabelOperation(before[field], patch[field])
          : patch[field]
      }
      if (!email.external && typeof after.folder === 'string') {
        after.folder = await this.categoryMappings.pathFor(ownerEmail, email.domain, after.folder) || after.folder
      }
      if (sameState(before, after)) continue
      changes.push({
        id: email.id, domain: email.domain, before, after,
        ...(email.external ? {
          source: 'provider', accountId: email.providerAccountId, stableId: email.providerStableId,
          providerMutations: { before: [], after: [] },
          providerGenerations: { before: 0, after: 0 },
        } : {}),
      })
    }
    return changes
  }

  async _writeTarget(changes, target, ownerEmail, domains, providerToken = '', actionId = '') {
    const authorized = await this._authorizeMessages(changes.map(change => change.id), ownerEmail, domains, providerToken)
    const currentById = new Map(authorized.map(message => [message.id, message]))
    const updated = []
    for (const [index, change] of changes.entries()) {
      if (change.source === 'provider') {
        if (!providerToken) throw new MailActionError('Connected account authorization is unavailable', 503)
        let loaded = await loadExternalMessageMetadata(providerToken, change.id)
        if (!loaded || loaded.identity.accountId !== change.accountId || loaded.identity.stableId !== change.stableId) {
          throw new MailActionError('Email not found', 404)
        }
        const opposite = target === 'before' ? 'after' : 'before'
        const mustOrderInverse = Array.isArray(change.providerMutations?.[opposite]) && change.providerMutations[opposite].length > 0
        const current = currentById.get(change.id)
        if (mustOrderInverse || !matchesState(current, change[target])) {
          let mutations
          const generation = Number(change.providerGenerations?.[target] || 0) + 1
          try {
            mutations = await queueExternalMessagePatch(providerToken, loaded, change[target], {
              idempotencyPrefix: `mail-action-${actionId}-${target}-${generation}-${index}`,
            })
          } catch (error) {
            if (error instanceof ExternalFolderError) throw new MailActionError(error.message)
            throw error
          }
          change.providerMutations ||= { before: [], after: [] }
          change.providerGenerations ||= { before: 0, after: 0 }
          change.providerMutations[target] = mutations.map(mutation => mutation.id)
          change.providerGenerations[target] = generation
          await this._updateJournal(actionId, { changesJson: JSON.stringify(changes), updatedAt: Date.now() }, ownerEmail)
          const move = mutations.find(mutation => mutation?.kind === 'move')
          if (move) {
            const completed = await waitForProviderMutation(providerToken, change.accountId, move.id)
            const replacement = Array.isArray(completed?.resultStableIds) ? completed.resultStableIds[0] : ''
            const id = externalMessageId(change.accountId, replacement)
            if (!id) throw new MailActionError('The connected account did not return a safe destination identity', 503)
            change.stableId = replacement
            change.id = id
            await this._updateJournal(actionId, { changesJson: JSON.stringify(changes), updatedAt: Date.now() }, ownerEmail)
            loaded = await loadExternalMessageMetadata(providerToken, id)
            if (!loaded) throw new MailActionError('The moved provider message is not yet available', 503)
          }
          const projected = projectExternalPatch(loaded, change[target])
          const message = projected ? presentExternalSummary(projected.indexed) : null
          if (!message) throw new MailActionError('Provider action projection failed', 503)
          updated.push(message)
        } else {
          updated.push(current)
        }
        continue
      }
      const [docId, email] = await this.emails.findById(change.id)
      if (!docId || !email || !domains.includes(email.domain) || !canAccessEmail(email, ownerEmail)) {
        throw new MailActionError('Email not found', 404)
      }
      if (!matchesState(email, change[target])) await this.emails.update(docId, change[target])
      updated.push({ ...email, ...change[target] })
    }
    return updated
  }

  async _rollbackPrepared(changes, ownerEmail, domains, providerToken = '', actionId = '') {
    try {
      if (changes.some(change => change.source === 'provider')) {
        await this._writeTarget(changes, 'before', ownerEmail, domains, providerToken, `${actionId}-rollback`)
        return true
      }
      for (const change of changes) {
        const [docId, email] = await this.emails.findById(change.id)
        if (!docId || !email || !domains.includes(email.domain) || !canAccessEmail(email, ownerEmail)) return false
        if (matchesState(email, change.after)) await this.emails.update(docId, change.before)
        else if (!matchesState(email, change.before)) return false
      }
      return true
    } catch {
      return false
    }
  }

  async _statesAccepted(messages, changes, target, providerToken = '') {
    for (const [index, change] of changes.entries()) {
      const message = messages[index]
      if (change.source !== 'provider') {
        if (!matchesState(message, change[target])) return false
        continue
      }
      if (!providerToken) throw new MailActionError('Connected account authorization is unavailable', 503)
      const mutationIds = Array.isArray(change.providerMutations?.[target]) ? change.providerMutations[target] : []
      if (mutationIds.length === 0) {
        if (!matchesState(message, change[target])) return false
        continue
      }
      const states = await Promise.all(mutationIds.map(id => ExternalAccounts.mutation(providerToken, change.accountId, id)))
      if (states.some(mutation => mutation?.state === 'failed')) {
        throw new MailActionError('The connected account rejected this action. Refresh to review the provider version.', 409)
      }
      if (states.every(mutation => mutation?.state === 'applied') && !matchesState(message, change[target])) {
        throw new MailActionError('The connected account changed after this action. Refresh before choosing another result.', 409)
      }
    }
    return true
  }

  async _refreshProviderIdentities(changes, target, providerToken, actionId, ownerEmail) {
    if (!providerToken) return
    let changed = false
    for (const change of changes) {
      if (change.source !== 'provider') continue
      const mutationIds = Array.isArray(change.providerMutations?.[target]) ? change.providerMutations[target] : []
      for (const mutationId of mutationIds) {
        const mutation = await ExternalAccounts.mutation(providerToken, change.accountId, mutationId)
        if (mutation?.kind !== 'move' || mutation?.state !== 'applied') continue
        const replacement = Array.isArray(mutation.resultStableIds) ? mutation.resultStableIds[0] : ''
        const id = externalMessageId(change.accountId, replacement)
        if (!id) throw new MailActionError('The connected account move journal is missing its destination identity', 503)
        if (change.stableId !== replacement || change.id !== id) {
          change.stableId = replacement
          change.id = id
          changed = true
        }
      }
    }
    if (changed) await this._updateJournal(actionId, { changesJson: JSON.stringify(changes), updatedAt: Date.now() }, ownerEmail)
  }

  async _updateJournal(id, patch, ownerEmail = '') {
    if (!await this.actions.updateById(id, patch, ownerEmail)) {
      throw new MailActionError('Mail action journal became unavailable', 503)
    }
  }

  _serial(ownerEmail, task) {
    const key = ownerEmail.toLowerCase()
    const previous = this._tails.get(key) || Promise.resolve()
    const bound = AsyncLocalStorage.bind(task)
    const result = previous.then(bound, bound)
    const tail = result.then(() => undefined, () => undefined)
    this._tails.set(key, tail)
    void tail.finally(() => {
      if (this._tails.get(key) === tail) this._tails.delete(key)
    })
    return result
  }
}

async function waitForProviderMutation(token, accountId, mutationId) {
  const deadline = Date.now() + PROVIDER_MUTATION_TIMEOUT_MS
  while (Date.now() < deadline) {
    const mutation = await ExternalAccounts.mutation(token, accountId, mutationId)
    if (mutation?.state === 'applied') return mutation
    if (mutation?.state === 'failed') {
      throw new MailActionError('The connected account rejected this action. Refresh to review the provider version.', 409)
    }
    await new Promise(resolve => setTimeout(resolve, PROVIDER_MUTATION_POLL_MS))
  }
  throw new MailActionError('The connected account action is still reconciling. Try again shortly.', 503)
}

function parsePatch(input) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) throw new MailActionError('patch is required')
  const source = /** @type {Record<string, unknown>} */ (input)
  const unknown = Object.keys(source).filter(key => !MUTABLE_FIELDS.includes(key))
  if (unknown.length > 0) throw new MailActionError(`Unsupported patch field: ${unknown[0]}`)
  const patch = {}
  if ('folder' in source) {
    if (typeof source.folder !== 'string' || !source.folder.trim()) throw new MailActionError('folder must be a non-empty string')
    const folder = normalizeCategoryPath(source.folder)
    if (!folder) throw new MailActionError('folder must be a valid category path')
    patch.folder = folder
  }
  if ('labels' in source) {
    const result = parseLabelOperation(source.labels)
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

function parseChanges(record) {
  let value
  try { value = JSON.parse(record.changesJson) } catch { throw new MailActionError('Mail action record is corrupt', 503) }
  if (!Array.isArray(value) || value.length === 0 || value.length > MAX_ACTION_MESSAGES) {
    throw new MailActionError('Mail action record is corrupt', 503)
  }
  for (const change of value) {
    if (!change || typeof change !== 'object' || typeof change.id !== 'string' || typeof change.domain !== 'string') {
      throw new MailActionError('Mail action record is corrupt', 503)
    }
    validateStoredState(change.before)
    validateStoredState(change.after)
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
        if (!Array.isArray(ids) || ids.length > MUTABLE_FIELDS.length || ids.some(id => !/^mut_[A-Za-z0-9_-]{20,80}$/.test(id))) {
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

function validateStoredState(state) {
  if (!state || typeof state !== 'object' || Array.isArray(state)) throw new MailActionError('Mail action record is corrupt', 503)
  const fields = Object.keys(state)
  if (fields.length === 0 || fields.some(field => !MUTABLE_FIELDS.includes(field))) throw new MailActionError('Mail action record is corrupt', 503)
  if ('folder' in state && (typeof state.folder !== 'string' || !state.folder)) throw new MailActionError('Mail action record is corrupt', 503)
  if ('labels' in state && (!Array.isArray(state.labels) || state.labels.length > 100 || normalizeStoredLabels(state.labels).length !== state.labels.length)) {
    throw new MailActionError('Mail action record is corrupt', 503)
  }
  if ('read' in state && typeof state.read !== 'boolean') throw new MailActionError('Mail action record is corrupt', 503)
  if ('starred' in state && typeof state.starred !== 'boolean') throw new MailActionError('Mail action record is corrupt', 503)
}

function ensureDomains(changes, domains) {
  if (changes.some(change => change.source !== 'provider' && !domains.includes(change.domain))) {
    throw new MailActionError('Mail action not found', 404)
  }
}

function fieldValue(email, field) {
  if (field === 'folder') return email.folder || 'inbox'
  if (field === 'labels') return normalizeStoredLabels(email.labels)
  return email[field] === true
}

function matchesState(email, state) {
  return Object.entries(state).every(([field, value]) => equalValue(fieldValue(email, field), value))
}

function sameState(left, right) {
  const keys = Object.keys(left)
  return keys.length === Object.keys(right).length && keys.every(key => equalValue(left[key], right[key]))
}

function equalValue(left, right) {
  if (!Array.isArray(left) || !Array.isArray(right)) return left === right
  return left.length === right.length && left.every((value, index) => value === right[index])
}

/**
 * @param {Record<string, string | boolean>} patch
 * @param {number} count
 * @returns {import('../repositories/MailActionsRepository.js').MailActionEntry['kind']}
 */
function actionKind(patch, count) {
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

function actionResult(actionId, record, status, emails) {
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
      labels: normalizeStoredLabels(email.labels),
      read: email.read === true,
      starred: email.starred === true,
      ...(email.external ? { external: true, providerAccountId: email.providerAccountId } : {}),
    })),
  }
}

/** @param {string[]} ids @param {Record<string, string|boolean>} patch */
function requestFingerprint(ids, patch) {
  const normalizedPatch = Object.fromEntries(Object.entries(patch).sort(([left], [right]) => left.localeCompare(right)))
  return createHash('sha256').update(JSON.stringify({ ids, patch: normalizedPatch })).digest('hex')
}
