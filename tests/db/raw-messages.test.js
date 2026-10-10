import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { createHash } from 'node:crypto'
import { DatabaseRepository, Buckets } from '@/shared/persistence/Fylo.js'
import { RawMessagesRepository } from '@/features/mailbox/repositories/RawMessagesRepository.js'

let fylo
let rawMessages
let testRoot
let previousFyloRoot

beforeEach(async () => {
  testRoot = mkdtempSync(join(tmpdir(), 'caduceus-raw-message-test-'))
  previousFyloRoot = process.env.FYLO_ROOT
  process.env.FYLO_ROOT = testRoot
  fylo = await DatabaseRepository.createDb(testRoot)
  rawMessages = new RawMessagesRepository(fylo)
})

afterEach(async () => {
  await fylo?.close?.()
  if (previousFyloRoot === undefined) delete process.env.FYLO_ROOT
  else process.env.FYLO_ROOT = previousFyloRoot
  rmSync(testRoot, { recursive: true, force: true })
})

describe('raw RFC 822 storage', () => {
  it('preserves exact source bytes and a verifiable digest', async () => {
    const source = 'From: sender@example.net\r\nSubject: Héllo\r\n\r\nExact body\r\n'
    const record = await rawMessages.saveForEmail('raw-1', 'example.com', source)
    expect(record.emailId).toBe('raw-1')
    expect(record.size).toBe(Buffer.byteLength(source))
    expect(record.sha256).toBe(createHash('sha256').update(source).digest('hex'))

    const found = await rawMessages.findForEmail('raw-1')
    expect(found?.bucketId).toBeString()
    expect(Buffer.from(await rawMessages.readContent(found)).toString()).toBe(source)
    const stored = await fylo.getDoc(Buckets.RAW_MESSAGE_FILES, found.bucketId)
    expect(stored[found.bucketId].key).toMatch(/\/emails\/[a-f0-9]{64}\/source\.eml$/)
  })

  it('replaces an existing source and removes both records on delete', async () => {
    const first = await rawMessages.saveForEmail('raw-2', 'example.com', 'first')
    const second = await rawMessages.saveForEmail('raw-2', 'example.com', 'second')
    expect(second.bucketId).not.toBe(first.bucketId)
    await expect(fylo.getFileBytes(Buckets.RAW_MESSAGE_FILES, first.bucketId)).rejects.toThrow()

    await rawMessages.deleteForEmail('raw-2')
    expect(await rawMessages.findForEmail('raw-2')).toBeNull()
    await expect(fylo.getFileBytes(Buckets.RAW_MESSAGE_FILES, second.bucketId)).rejects.toThrow()
  })
})
