import { afterEach, beforeEach, describe, expect, it } from 'bun:test'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { DatabaseRepository } from '@/shared/persistence/DatabaseRepository.js'
import { AttachmentsRepository } from '@/features/mailbox/repositories/AttachmentsRepository.js'
import { EmailsRepository } from '@/features/mailbox/repositories/EmailsRepository.js'
import { FollowUpRemindersRepository } from '@/features/delivery/repositories/FollowUpRemindersRepository.js'
import { FollowUpReminderService, threadKey } from '@/features/delivery/services/FollowUpReminderService.js'

let root, fylo, emails, reminders, service

beforeEach(async () => {
  root = mkdtempSync(join(tmpdir(), 'caduceus-reminders-'))
  fylo = await DatabaseRepository.createDb(root)
  emails = new EmailsRepository(fylo, new AttachmentsRepository(fylo))
  reminders = new FollowUpRemindersRepository(fylo)
  service = new FollowUpReminderService(reminders, emails)
})

afterEach(async () => {
  await fylo?.close()
  rmSync(root, { recursive: true, force: true })
})

const input = (over = {}) => ({
  ownerEmail: 'alice@example.com',
  emailId: 'sent-1',
  recipient: 'bob@other.com',
  subject: 'Quarterly plan',
  remindAt: Date.now() + 60_000,
  ...over,
})

describe('FollowUpReminderService', () => {
  it('normalizes reply and forward subject prefixes into one thread key', () => {
    expect(threadKey('Re: Fwd: Quarterly Plan')).toBe('quarterly plan')
  })

  it('updates an existing active reminder instead of duplicating it', async () => {
    const first = await service.schedule(input())
    const nextTime = Date.now() + 120_000
    const second = await service.schedule(input({ remindAt: nextTime }))
    expect(second.id).toBe(first.id)
    expect(await reminders.listForOwner('alice@example.com')).toHaveLength(1)
    expect((await reminders.listForOwner('alice@example.com'))[0].remindAt).toBe(nextTime)
  })

  it('clears a pending reminder when the recipient replies on the thread', async () => {
    const reminder = await service.schedule(input())
    expect(await service.resolveForReply({
      recipient: 'ALICE@example.com', sender: 'BOB@other.com', subject: 'Re: Quarterly plan',
    })).toBe(1)
    expect((await reminders.findById(reminder.id))[1].status).toBe('completed')
  })

  it('does not clear a reminder for an unrelated sender or subject', async () => {
    const reminder = await service.schedule(input())
    expect(await service.resolveForReply({
      recipient: 'alice@example.com', sender: 'carol@other.com', subject: 'Quarterly plan',
    })).toBe(0)
    expect((await reminders.findById(reminder.id))[1].status).toBe('scheduled')
  })

  it('uses RFC thread identity instead of an equal subject when both sides provide it', async () => {
    const reminder = await service.schedule(input({ threadId: 'a'.repeat(64) }))
    expect(await service.resolveForReply({
      recipient: 'alice@example.com', sender: 'bob@other.com', subject: 'Re: Quarterly plan', threadId: 'b'.repeat(64),
    })).toBe(0)
    expect((await reminders.findById(reminder.id))[1].status).toBe('scheduled')
    expect(await service.resolveForReply({
      recipient: 'alice@example.com', sender: 'bob@other.com', subject: 'Changed subject', threadId: 'a'.repeat(64),
    })).toBe(1)
  })

  it('resurfaces a due reminder once as an unread inbox message', async () => {
    const reminder = await service.schedule(input({ remindAt: Date.now() - 1 }))
    expect(await service.processDue('alice@example.com')).toBe(1)
    expect(await service.processDue('alice@example.com')).toBe(0)

    const [, stored] = await reminders.findById(reminder.id)
    expect(stored.status).toBe('due')
    const [, notice] = await emails.findById(stored.noticeEmailId)
    expect(notice.folder).toBe('inbox')
    expect(notice.read).toBe(false)
    expect(notice.subject).toBe('Re: Quarterly plan')
    expect(notice.ownerEmail).toBe('alice@example.com')
  })

  it('processes every mailbox independently and reports bounded pass metrics', async () => {
    await service.schedule(input({ emailId: 'sent-all-alice', remindAt: Date.now() - 10 }))
    await service.schedule(input({
      ownerEmail: 'bob@example.net', emailId: 'sent-all-bob', recipient: 'alice@other.com', remindAt: Date.now() - 10,
    }))
    expect(await service.processDueAll()).toEqual({ due: 2, surfaced: 2, reconciled: 0, failed: 0 })
    expect(await service.processDueAll()).toEqual({ due: 0, surfaced: 0, reconciled: 0, failed: 0 })
    expect((await emails.findById('follow-up-' + (await reminders.listForOwner('bob@example.net'))[0].id))[1])
      .toMatchObject({ ownerEmail: 'bob@example.net', recipient: 'bob@example.net', folder: 'inbox', read: false })
  })

  it('only lets the owning mailbox dismiss a reminder', async () => {
    const reminder = await service.schedule(input())
    expect(await service.dismiss(reminder.id, 'mallory@example.com')).toBe(false)
    expect(await service.dismiss(reminder.id, 'alice@example.com')).toBe(true)
    expect((await reminders.findById(reminder.id))[1].status).toBe('dismissed')
  })
})
