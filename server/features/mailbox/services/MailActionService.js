import { createHash, randomUUID } from 'node:crypto'
import { AsyncLocalStorage } from 'node:async_hooks'
import { EmailsRecords } from '../../../shared/mailbox/EmailsRecords.js'
import { CategoryPath } from '../../../shared/mailbox/CategoryPath.js'
import { MailActionLabels } from '../../../shared/mailbox/MailActionLabels.js'
import { ExternalMessage } from '../../../shared/mailbox/ExternalMessage.js'
import { UnifiedMail } from '../../../shared/mailbox/UnifiedMail.js'
import { ExternalAccounts } from '../../../shared/accounts/ExternalAccounts.js'
import { ExternalFolderError } from '../../../shared/mailbox/ExternalFolderError.js'
import { MailActionRules } from '../../../shared/mailbox/MailActionRules.js'
import { MailActionError } from '../../../shared/errors/MailActionError.js'

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

  /** @param {{ids:string[],patch:Record<string, any>,clientRequestId?:string,ownerEmail:string,domains:string[],providerToken?:string,now?:number}} input */
  async apply(input) {
    return await this._serial(input.ownerEmail, async () => {
      await this._reconcileOwner(input.ownerEmail, input.domains, input.providerToken)
      const now = input.now ?? Date.now()
      const fingerprint = MailActionRules.requestFingerprint(input.ids, input.patch)
      if (input.clientRequestId) {
        const [existingId, existing] = await this.actions.findByClientRequestId(input.ownerEmail, input.clientRequestId)
        if (existingId && existing) {
          if (existing.requestFingerprint !== fingerprint) {
            throw new MailActionError('clientRequestId was already used for another action', 409)
          }
          const changes = MailActionRules.parseChanges(existing)
          MailActionRules.ensureDomains(changes, input.domains)
          const messages = await this._authorizeMessages(changes.map(change => change.id), input.ownerEmail, input.domains, input.providerToken)
          const target = existing.status === 'undone' ? 'before' : 'after'
          if (!await this._statesAccepted(messages, changes, target, input.providerToken)) {
            throw new MailActionError('Messages changed after this action; no changes were overwritten', 409)
          }
          return MailActionRules.actionResult(existingId, existing, existing.status, messages)
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
        kind: MailActionRules.actionKind(input.patch, changes.length),
        status: /** @type {const} */ ('prepared'),
        changesJson: JSON.stringify(changes),
        createdAt: now,
        updatedAt: now,
        expiresAt: now + MailActionRules.ACTION_TTL_MS,
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
        return MailActionRules.actionResult(actionId, record, 'applied', updated)
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
      if (!MailActionRules.INCOMPLETE_STATUSES.has(record.status)) continue
      let changes
      try {
		changes = MailActionRules.parseChanges(record)
		MailActionRules.ensureDomains(changes, domains)
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
    const changes = MailActionRules.parseChanges(record)
    MailActionRules.ensureDomains(changes, domains)
    if (record.expiresAt < transition.now) throw new MailActionError('The undo window has expired', 409)
    if (record.status === transition.to) {
      const current = await this._authorizeMessages(changes.map(change => change.id), ownerEmail, domains, providerToken)
      if (await this._statesAccepted(current, changes, transition.target, providerToken)) {
        return MailActionRules.actionResult(id, record, transition.to, current)
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
    return MailActionRules.actionResult(id, record, transition.to, updated)
  }

  async _authorizeMessages(ids, ownerEmail, domains, providerToken = '') {
    const messages = []
    for (const id of ids) {
      if (ExternalMessage.parseExternalMessageId(id)) {
        if (!providerToken) throw new MailActionError('Connected account authorization is unavailable', 503)
        const loaded = await UnifiedMail.loadExternalMessageMetadata(providerToken, id)
        const message = loaded ? ExternalMessage.presentExternalSummary(loaded.indexed) : null
        if (!loaded || !message) throw new MailActionError('Email not found', 404)
        messages.push(message)
        continue
      }
      const [, email] = await this.emails.findById(id)
      if (!email || !domains.includes(email.domain) || !EmailsRecords.canAccessEmail(email, ownerEmail)) {
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
      for (const field of MailActionRules.MUTABLE_FIELDS) {
        if (!(field in patch)) continue
        before[field] = MailActionRules.fieldValue(email, field)
        after[field] = field === 'labels'
          ? MailActionLabels.applyLabelOperation(before[field], patch[field])
          : patch[field]
      }
      if (!email.external && typeof after.folder === 'string') {
        after.folder = await this.categoryMappings.pathFor(ownerEmail, email.domain, after.folder) || after.folder
      }
      if (MailActionRules.sameState(before, after)) continue
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
        let loaded = await UnifiedMail.loadExternalMessageMetadata(providerToken, change.id)
        if (!loaded || loaded.identity.accountId !== change.accountId || loaded.identity.stableId !== change.stableId) {
          throw new MailActionError('Email not found', 404)
        }
        const opposite = target === 'before' ? 'after' : 'before'
        const mustOrderInverse = Array.isArray(change.providerMutations?.[opposite]) && change.providerMutations[opposite].length > 0
        const current = currentById.get(change.id)
        if (mustOrderInverse || !MailActionRules.matchesState(current, change[target])) {
          let mutations
          const generation = Number(change.providerGenerations?.[target] || 0) + 1
          try {
            mutations = await UnifiedMail.queueExternalMessagePatch(providerToken, loaded, change[target], {
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
            const completed = await MailActionRules.waitForProviderMutation(providerToken, change.accountId, move.id)
            const replacement = Array.isArray(completed?.resultStableIds) ? completed.resultStableIds[0] : ''
            const id = ExternalMessage.externalMessageId(change.accountId, replacement)
            if (!id) throw new MailActionError('The connected account did not return a safe destination identity', 503)
            change.stableId = replacement
            change.id = id
            await this._updateJournal(actionId, { changesJson: JSON.stringify(changes), updatedAt: Date.now() }, ownerEmail)
            loaded = await UnifiedMail.loadExternalMessageMetadata(providerToken, id)
            if (!loaded) throw new MailActionError('The moved provider message is not yet available', 503)
          }
          const projected = UnifiedMail.projectExternalPatch(loaded, change[target])
          const message = projected ? ExternalMessage.presentExternalSummary(projected.indexed) : null
          if (!message) throw new MailActionError('Provider action projection failed', 503)
          updated.push(message)
        } else {
          updated.push(current)
        }
        continue
      }
      const [docId, email] = await this.emails.findById(change.id)
      if (!docId || !email || !domains.includes(email.domain) || !EmailsRecords.canAccessEmail(email, ownerEmail)) {
        throw new MailActionError('Email not found', 404)
      }
      if (!MailActionRules.matchesState(email, change[target])) await this.emails.update(docId, change[target])
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
        if (!docId || !email || !domains.includes(email.domain) || !EmailsRecords.canAccessEmail(email, ownerEmail)) return false
        if (MailActionRules.matchesState(email, change.after)) await this.emails.update(docId, change.before)
        else if (!MailActionRules.matchesState(email, change.before)) return false
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
        if (!MailActionRules.matchesState(message, change[target])) return false
        continue
      }
      if (!providerToken) throw new MailActionError('Connected account authorization is unavailable', 503)
      const mutationIds = Array.isArray(change.providerMutations?.[target]) ? change.providerMutations[target] : []
      if (mutationIds.length === 0) {
        if (!MailActionRules.matchesState(message, change[target])) return false
        continue
      }
      const states = await Promise.all(mutationIds.map(id => ExternalAccounts.mutation(providerToken, change.accountId, id)))
      if (states.some(mutation => mutation?.state === 'failed')) {
        throw new MailActionError('The connected account rejected this action. Refresh to review the provider version.', 409)
      }
      if (states.every(mutation => mutation?.state === 'applied') && !MailActionRules.matchesState(message, change[target])) {
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
        const id = ExternalMessage.externalMessageId(change.accountId, replacement)
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

