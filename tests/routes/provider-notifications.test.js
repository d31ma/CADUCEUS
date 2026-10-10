import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { createHmac } from 'node:crypto'
import { setupRoutes, teardownRoutes } from './helpers.js'
import { Handler as ProviderNotificationTick } from '@/routes/notifications/provider/tick/yon.js'
import { ExternalAccounts } from '@/platform/accounts/ExternalAccounts.js'

let app
const originals = {
  claim: ExternalAccounts.claimNotifications,
  complete: ExternalAccounts.completeNotifications,
}

beforeAll(async () => { app = await setupRoutes() }, 300_000)
afterAll(async () => {
  ExternalAccounts.claimNotifications = originals.claim
  ExternalAccounts.completeNotifications = originals.complete
  await teardownRoutes()
})

function signedRequest() {
  const body = {}
  const secret = process.env.INBOUND_WEBHOOK_SECRET || ''
  return { body, headers: { 'x-caduceus-signature': createHmac('sha256', secret).update(JSON.stringify(body)).digest('hex') } }
}

describe('durable provider new-mail notification tick', () => {
  it('requires the internal HMAC and settles delivered provider alerts', async () => {
    expect((await ProviderNotificationTick.POST({ body: {}, headers: {} })).unauthorized).toBe(true)
    const completed = []
    ExternalAccounts.claimNotifications = async () => ({
      token: 'claim-token',
      notifications: [{
        id: 'pnot_abcdefghijklmnopqrstuvwx',
        accountId: 'ext_AAAAAAAAAAAAAAAAAAAA',
        ownerAddress: 'alice@example.com',
        stableId: 'INBOX:77:10',
        sender: 'remote@example.net',
        subject: 'Provider alert',
        receivedAt: new Date().toISOString(),
      }],
    })
    ExternalAccounts.completeNotifications = async (...args) => { completed.push(args); return { completed: true } }
    app.push.notifyProviderNewEmail = async notification => notification.subject === 'Provider alert'

    expect(await ProviderNotificationTick.POST(signedRequest())).toEqual({ claimed: 1, delivered: 1, retried: 0 })
    expect(completed).toEqual([['claim-token', ['pnot_abcdefghijklmnopqrstuvwx'], true]])
  })

  it('returns failed transports to the durable retry queue', async () => {
    const completed = []
    ExternalAccounts.claimNotifications = async () => ({
      token: 'retry-token',
      notifications: [{ id: 'pnot_abcdefghijklmnopqrstuvwx', subject: 'retry' }],
    })
    ExternalAccounts.completeNotifications = async (...args) => { completed.push(args); return { completed: true } }
    app.push.notifyProviderNewEmail = async () => false
    expect(await ProviderNotificationTick.POST(signedRequest())).toEqual({ claimed: 1, delivered: 0, retried: 1 })
    expect(completed).toEqual([['retry-token', ['pnot_abcdefghijklmnopqrstuvwx'], false, 'push_delivery_failed']])
  })
})
