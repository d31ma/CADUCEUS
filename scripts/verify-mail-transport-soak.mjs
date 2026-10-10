import assert from 'node:assert/strict'
import { OutboundQueueService } from '../server/features/delivery/services/outbound-queue.js'

const HOUR_MS = 60 * 60_000
const START = Date.now()
const HOURS = 72
const BATCH_SIZE = 16

class MemoryOutbox {
  constructor() { this.entries = new Map() }
  async put(entry) { this.entries.set(entry.id, structuredClone(entry)); return entry.id }
  async delete(id) { this.entries.delete(id) }
  async update(id, patch) { this.entries.set(id, { ...this.entries.get(id), ...structuredClone(patch) }) }
  async listAll() { return Object.fromEntries(this.entries) }
  async findById(id) { const entry = this.entries.get(id); return entry ? [id, structuredClone(entry)] : [null, null] }
  async findDue(now) {
    return Object.fromEntries([...this.entries].filter(([, entry]) => (entry.nextAttempt ?? entry.expiresAt) <= now))
  }
}

class DeterministicSmtpPeer {
  constructor() { this.attempts = new Map() }
  async sendEmail(_sender, email) {
    const recipients = email.envelopeRecipients || [...(email.to || []), ...(email.cc || []), ...(email.bcc || [])]
    const results = recipients.map(recipient => this.deliveryResult(recipient))
    if (results.some(result => result.failedRecipients.length)) {
      throw Object.assign(new Error('deterministic remote response'), { deliveryResults: results })
    }
    return { messageId: `soak-${this.attempts.size}` }
  }
  deliveryResult(recipient) {
    const attempts = (this.attempts.get(recipient) || 0) + 1
    this.attempts.set(recipient, attempts)
    if (recipient.startsWith('gone+')) return failed(recipient, 550)
    if (recipient.startsWith('temp+') && attempts <= 2) return failed(recipient, 451)
    return { deliveredRecipients: [recipient], failedRecipients: [], responseCode: 250 }
  }
}

const failed = (recipient, responseCode) => ({ deliveredRecipients: [], failedRecipients: [recipient], responseCode })
const outbox = new MemoryOutbox()
const smtp = new DeterministicSmtpPeer()
const bounces = []
const suppressed = new Set()
const queue = new OutboundQueueService(
  /** @type {any} */ (outbox),
  /** @type {any} */ (smtp),
  /** @type {any} */ ({ async put(email) { bounces.push(email); return email.id } }),
  undefined,
  undefined,
  /** @type {any} */ ({ async pathFor() { return 'inbox' } }),
  /** @type {any} */ ({ async suppress(recipient) { suppressed.add(recipient) } }),
)

const totals = { delivered: 0, retried: 0, bounced: 0 }
for (let minute = 0; minute < HOURS * 60 + 30; minute++) {
  const now = START + minute * 60_000
  if (minute < HOURS * 60 && minute % 60 === 0) {
    const hour = minute / 60
    for (let index = 0; index < BATCH_SIZE; index++) {
      const id = `${hour}-${index}`
      await queue.enqueue('owner@local.test', {
        to: [`ok+${id}@accept.test`, `temp+${id}@grey.test`, `gone+${id}@reject.test`],
        subject: `72-hour soak ${id}`,
        text: 'bounded deterministic transport payload',
      }, { id: `soak-${id}`, availableAt: now })
    }
  }
  const result = await queue.processDue(now)
  totals.delivered += result.delivered
  totals.retried += result.retried
  totals.bounced += result.bounced
  const health = await queue.status(now)
  assert.ok(health.depth >= 0 && health.due >= 0 && health.due <= health.depth)
}

const submissions = HOURS * BATCH_SIZE
assert.equal((await queue.status(START + (HOURS * 60 + 30) * 60_000)).depth, 0)
assert.deepEqual(totals, { delivered: submissions, retried: submissions * 2, bounced: submissions })
assert.equal(bounces.length, submissions)
assert.equal(suppressed.size, submissions)
for (const [recipient, attempts] of smtp.attempts) {
  if (recipient.startsWith('ok+')) assert.equal(attempts, 1, `${recipient} was delivered more than once`)
  if (recipient.startsWith('temp+')) assert.equal(attempts, 3, `${recipient} did not follow the retry plan`)
  if (recipient.startsWith('gone+')) assert.equal(attempts, 1, `${recipient} was retried after a permanent failure`)
}

process.stdout.write(`${JSON.stringify({
  virtualHours: HOURS,
  submissions,
  smtpAttempts: smtp.attempts.size,
  ...totals,
  duplicateAcceptedDeliveries: 0,
  remainingQueueDepth: 0,
})}\n`)
