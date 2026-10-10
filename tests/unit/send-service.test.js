import { describe, expect, it } from 'bun:test'
import { SendService } from '@/features/delivery/services/send.js'

const request = {
  to: ['recipient@other.test'],
  subject: 'Persistence boundary',
  text: 'This message was accepted by SMTP.',
}

describe('SendService immediate-delivery boundary', () => {
  it('never queues a duplicate after SMTP succeeds but Sent-copy persistence fails', async () => {
    let enqueueCount = 0
    const service = new SendService(
      { put: async () => {} },
      { saveForEmail: async () => {} },
      { saveForEmail: async () => { throw new Error('FYLO unavailable') } },
      {},
      {},
      { enqueue: async () => { enqueueCount++; return 'queued-duplicate' } },
      { sendEmail: async () => ({ messageId: 'smtp-accepted' }) },
    )

    await expect(service.sendNow('owner@example.test', 'sender@example.test', request, 'tracking-1'))
      .rejects.toThrow('FYLO unavailable')
    expect(enqueueCount).toBe(0)
  })
})
