import { afterAll, beforeAll, describe, expect, test } from 'bun:test'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { Fylo } from '../../server/deps/fylo.mjs'

const binary = process.env.FYLO_BINARY
const supportsPosixIdentity = process.platform !== 'win32'
  && typeof process.getuid === 'function'
  && typeof process.getgid === 'function'

describe.skipIf(!binary || !supportsPosixIdentity)('local FYLO issue-68 bridge', () => {
  let root
  let source
  let environment
  let fylo

  beforeAll(async () => {
    root = mkdtempSync(join(tmpdir(), 'caduceus-fylo-access-'))
    source = join(root, 'attachment.txt')
    environment = join(root, 'fylo.env')
    writeFileSync(source, 'private attachment')
    writeFileSync(environment, 'CADUCEUS_FYLO_SHIM_TEST="explicit-env-file"\n')
    fylo = new Fylo(root, { binary, env: environment, shardWidth: 2 })
    await fylo.createCollection('messages')
    await fylo.createCollection('attachments', 'file')
  })

  afterAll(async () => {
    await fylo?.close()
    if (root) await rm(root, { recursive: true, force: true })
  })

  test('enforces trusted virtual groups for documents and attachment bytes', async () => {
    const gid = process.getgid()
    const memberUid = process.getuid() + 10_001
    const member = { uid: memberUid, groups: [gid] }
    const outsider = { uid: memberUid + 1, groups: [] }

    const documentId = await fylo
      .putData('messages', { domain: 'example.com', subject: 'private' })
      .as({ gid, mode: 0o660 })

    const visible = await fylo
      .findDocs('messages', { $ops: [{ domain: { $eq: 'example.com' } }] })
      .as(member)
    expect(visible[documentId]?.subject).toBe('private')

    await expect(fylo.getDoc('messages', documentId).as(outsider)).rejects.toMatchObject({
      code: 'EACCES',
    })

    const attachmentId = await fylo
      .putFile('attachments', source, {
        key: '/example.com/attachment.txt',
        maxBytes: 64,
      })
      .as({ gid, mode: 0o660 })

    expect(new TextDecoder().decode(
      await fylo.getFileBytes('attachments', attachmentId).as(member),
    )).toBe('private attachment')

    await expect(
      fylo.getFileBytes('attachments', attachmentId).as(outsider),
    ).rejects.toMatchObject({ code: 'EACCES' })
  })
})
