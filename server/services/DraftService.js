import { FyloAccessContext } from '../shared/persistence/FyloAccessContext.js'
import { Threading } from '../shared/mailbox/Threading.js'
import { Security } from '../shared/security/Security.js'
/** @import { EmailsHelper } from '../helpers/EmailsHelper.js' */
/** @import { AttachmentsHelper } from '../helpers/AttachmentsHelper.js' */
/** @import { CategoryMappingService } from './CategoryMappingService.js' */

/** Durable, optimistic-concurrency draft lifecycle. */
@Service
export default class DraftService {
  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** Create once per client id, making a lost HTTP response safely retryable. */
  async create(ownerEmail, input) {
    const owner = ownerEmail.toLowerCase()
    const domain = owner.split('@')[1] || ''
    const draftFolder = await this.categoryMappingService?.pathFor(owner, domain, 'drafts') || 'drafts'
    const existing = (await this.emailsHelper.list([domain], { folder: draftFolder }))
      .find(email => email.draftOwner === owner && email.draftClientId === input.clientId)
    if (existing) return { draft: await this.present(existing), recovered: true }

    const id = `draft-${Security.randomText(16)}`
    const now = new Date().toISOString()
    const draft = this.#record(id, owner, domain, input, 1, now, draftFolder)
    return await FyloAccessContext.runForDomain(domain, async () => {
      const docId = await this.emailsHelper.put(draft)
      try {
        await this.attachmentsHelper.saveForEmail(id, domain, input.attachments)
      } catch (error) {
        await this.emailsHelper.delete(docId, id).catch(() => {})
        throw error
      }
      return { draft: await this.present(draft), recovered: false }
    })
  }

  /** Compare-and-swap update; stale tabs can never silently overwrite a draft. */
  async update(id, ownerEmail, allowedDomains, expectedVersion, input) {
    const found = await this.findOwned(id, ownerEmail, allowedDomains)
    if (!found) return { kind: 'not-found' }
    const { docId, draft } = found
    if (draft.draftClientId && draft.draftClientId !== input.clientId) return { kind: 'not-found' }
    const currentVersion = Number(draft.draftVersion || 0)
    if (currentVersion !== expectedVersion) {
      return { kind: 'conflict', currentVersion, updatedAt: draft.draftUpdatedAt || draft.receivedAt }
    }
    const nextVersion = currentVersion + 1
    const now = new Date().toISOString()
    const patch = this.#record(id, ownerEmail.toLowerCase(), draft.domain, input, nextVersion, now, draft.folder)
    await FyloAccessContext.runForDomain(draft.domain, async () => {
      await this.attachmentsHelper.replaceForEmail(id, draft.domain, input.attachments)
      await this.emailsHelper.update(docId, patch)
    })
    return { kind: 'updated', draft: await this.present(patch) }
  }

  async get(id, ownerEmail, allowedDomains) {
    const found = await this.findOwned(id, ownerEmail, allowedDomains)
    return found ? await this.present(found.draft) : null
  }

  async discard(id, ownerEmail, allowedDomains) {
    const found = await this.findOwned(id, ownerEmail, allowedDomains)
    if (!found) return false
    await FyloAccessContext.runForDomain(found.draft.domain, async () => {
      await this.emailsHelper.delete(found.docId, id)
    })
    return true
  }

  async findOwned(id, ownerEmail, allowedDomains) {
    const [docId, draft] = await this.emailsHelper.findById(id)
    if (!docId || !draft) return null
    const owner = ownerEmail.toLowerCase()
    const draftsPath = await this.categoryMappingService?.pathFor(owner, draft.domain, 'drafts') || 'drafts'
    if (draft.folder !== draftsPath) return null
    const recordedOwner = draft.draftOwner?.toLowerCase() || draft.sender.toLowerCase()
    if (recordedOwner !== owner || !allowedDomains.includes(draft.domain)) return null
    return { docId, draft }
  }

  /** Include attachment bytes only on the draft-specific authenticated API. */
  async present(draft) {
    const attachments = []
    for (const attachment of await this.attachmentsHelper.list(draft.id)) {
      const { docId: _docId, bucketId: _bucketId, storagePath: _storagePath, ...summary } = attachment
      attachments.push({
        ...summary,
        contentBase64: Buffer.from(await this.attachmentsHelper.readContent(attachment)).toString('base64'),
      })
    }
    return { ...draft, attachments }
  }

  #record(id, owner, domain, input, version, now, folder) {
    const references = input.references || []
    const root = references[0] || input.inReplyTo || `draft:${input.clientId}`
    return {
      id,
      domain,
      ownerEmail: owner,
      recipient: input.to[0] || owner,
      to: input.to,
      ...(input.cc.length ? { cc: input.cc } : {}),
      ...(input.bcc.length ? { bcc: input.bcc } : {}),
      sender: input.from || owner,
      ...(input.replyTo.length ? { replyTo: input.replyTo } : {}),
      subject: input.subject,
      body: input.text,
      ...(input.html ? { htmlBody: input.html } : {}),
      ...(input.inReplyTo ? { inReplyTo: input.inReplyTo } : {}),
      ...(input.sourceEmailId ? { sourceEmailId: input.sourceEmailId } : {}),
      ...(references.length ? { references } : {}),
      threadId: Threading.deriveThreadId(domain, root),
      folder,
      read: true,
      starred: false,
      receivedAt: now,
      processed: true,
      hasAttachments: input.attachments.length > 0,
      draftOwner: owner,
      draftClientId: input.clientId,
      draftVersion: version,
      draftUpdatedAt: now,
    }
  }
}
