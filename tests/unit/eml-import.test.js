import { describe, expect, it } from 'bun:test'
import { EmlImportService } from '../../server/features/mailbox/services/EmlImportService.js'

const parsed = {
  text: 'body',
  html: '',
  attachments: [{ filename: 'note.txt', contentType: 'text/plain', content: Buffer.from('note') }],
  headers: {
    from: 'Sender <sender@sender.test>',
    to: 'owner@example.com',
    subject: 'Imported',
    date: 'Fri, 14 Aug 2026 12:00:00 +0000',
    'message-id': '<imported@sender.test>',
  },
}

describe('EmlImportService', () => {
  it('commits the exact source-derived record without delivery side effects', async () => {
    const calls = []
    const service = new EmlImportService(
      { put: async email => { calls.push(['email', email]) } },
      {
        replaceForEmail: async (id, domain, attachments) => { calls.push(['attachments', id, domain, attachments]) },
        deleteForEmail: async () => {},
      },
      {
        saveForEmail: async (id, domain, source) => { calls.push(['source', id, domain, source]) },
        deleteForEmail: async () => {},
      },
      { parseMessage: async () => parsed },
    )
    const source = Buffer.from('From: sender@sender.test\r\n\r\nbody')
    const result = await service.importOne({ source, domain: 'example.com', mailbox: 'owner@example.com', folder: 'archive' })
    expect(result.subject).toBe('Imported')
    expect(calls.map(call => call[0])).toEqual(['source', 'attachments', 'email'])
    const email = calls[2][1]
    expect(email.folder).toBe('archive')
    expect(email.rfcMessageId).toBe('<imported@sender.test>')
    expect(email.receivedAt).toBe('2026-08-14T12:00:00.000Z')
  })

  it('removes staged attachment and source objects when the message row fails', async () => {
    const removed = []
    const service = new EmlImportService(
      { put: async () => { throw new Error('email write failed') } },
      {
        replaceForEmail: async () => {},
        deleteForEmail: async id => { removed.push(`attachments:${id}`) },
      },
      {
        saveForEmail: async () => {},
        deleteForEmail: async id => { removed.push(`source:${id}`) },
      },
      { parseMessage: async () => parsed },
    )
    await expect(service.importOne({
      source: Buffer.from('mail'), domain: 'example.com', mailbox: 'owner@example.com', folder: 'inbox',
    })).rejects.toThrow('email write failed')
    expect(removed).toHaveLength(2)
    expect(removed[0]).toStartWith('attachments:import-')
    expect(removed[1]).toStartWith('source:import-')
  })
})
