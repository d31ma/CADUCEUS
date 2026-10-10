import { canAccessEmail } from '../repositories/EmailsRepository.js'

export class ConversationArchiveError extends Error {
  /** @param {string} message @param {number} status */
  constructor(message, status = 400) { super(message); this.status = status }
}

/** Conversation-level archive with a bounded, undo-safe folder snapshot. */
export class ConversationArchiveService {
  /** @param {import('../repositories/EmailsRepository.js').EmailsRepository} emails @param {import('../repositories/ThreadsRepository.js').ThreadsRepository} threads @param {import('./CategoryMappingService.js').CategoryMappingService} categoryMappings */
  constructor(emails, threads, categoryMappings) { this.emails = emails; this.threads = threads; this.categoryMappings = categoryMappings }

  /** Resolve and authorize the source before a send is accepted. */
  async plan({ sourceEmailId, ownerEmail, domains }) {
    const [, selected] = await this.emails.findById(sourceEmailId)
    if (!selected || !domains.includes(selected.domain) || !canAccessEmail(selected, ownerEmail)) {
      throw new ConversationArchiveError('Source conversation not found', 404)
    }
    const thread = await this.threads.findForEmail(domains, sourceEmailId, ownerEmail)
    if (!thread) throw new ConversationArchiveError('Source conversation not found', 404)
    const terminalPaths = Object.fromEntries(await Promise.all(domains.map(async domain => [
      domain,
      await Promise.all(['archive', 'drafts', 'sent', 'trash'].map(role => this.categoryMappings?.pathFor(ownerEmail, domain, role))),
    ])))
    const messages = thread.emails
      .filter(email => !terminalPaths[email.domain]?.includes(email.folder || 'inbox'))
      .map(email => ({ id: email.id, domain: email.domain, folder: email.folder || 'inbox' }))
    if (!messages.length) throw new ConversationArchiveError('Source conversation is already archived', 409)
    return { sourceEmailId, threadId: thread.id, messages }
  }

  /** Archive the exact pre-authorized snapshot. */
  async archive(plan, ownerEmail = '') {
    let movedCount = 0
    for (const message of plan.messages) {
      const [docId, current] = await this.emails.findById(message.id)
      if (!docId || !current || current.folder !== message.folder) continue
      const archive = await this.categoryMappings?.pathFor(ownerEmail, message.domain, 'archive') || 'archive'
      await this.emails.update(docId, { folder: archive })
      movedCount += 1
    }
    if (movedCount !== plan.messages.length) throw new ConversationArchiveError('Source conversation changed before it could be archived', 409)
    return { archived: true, archivedCount: movedCount, archiveThreadId: plan.threadId }
  }

  serialize(plan) {
    return JSON.stringify({ sourceEmailId: plan.sourceEmailId, threadId: plan.threadId, messages: plan.messages })
  }

  /** Restore only rows that are still archived, preserving subsequent moves. */
  async restore(serialized, ownerEmail, domains) {
    let state
    try { state = JSON.parse(serialized || 'null') } catch { return 0 }
    if (!state || !Array.isArray(state.messages) || state.messages.length > 500) return 0
    const byFolder = new Map()
    for (const item of state.messages) {
      if (!item || typeof item.id !== 'string' || typeof item.folder !== 'string') continue
      const [, email] = await this.emails.findById(item.id)
      const archive = email ? await this.categoryMappings?.pathFor(ownerEmail, email.domain, 'archive') || 'archive' : 'archive'
      if (!email || email.folder !== archive || !domains.includes(email.domain) || !canAccessEmail(email, ownerEmail)) continue
      const ids = byFolder.get(item.folder) ?? []
      ids.push(email.id)
      byFolder.set(item.folder, ids)
    }
    let restored = 0
    for (const [folder, ids] of byFolder) restored += (await this.emails.updateByIds(ids, { folder })).length
    return restored
  }
}
