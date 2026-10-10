import { createHash, randomUUID } from 'node:crypto'

export class SendIdempotencyConflict extends Error {
  /** @param {string} message @param {'mismatch'|'applying'|'ambiguous'} kind */
  constructor(message, kind) { super(message); this.kind = kind }
}

/** Durable exactly-once HTTP acceptance boundary for outbound mail. */
export class SendIdempotencyService {
  /** @param {import('../repositories/send-requests.js').SendRequestsRepository} records */
  constructor(records) {
    this.records = records
    this._serial = Promise.resolve()
  }

  /** Stable hash independent of object property order. */
  hash(value) {
    return createHash('sha256').update(canonicalJson(value)).digest('hex')
  }

  /** Inspect without creating a journal row, so validation failures remain retryable. */
  async inspect(ownerEmail, idempotencyKey, requestHash) {
    const [documentId, record] = await this.records.find(ownerEmail, idempotencyKey)
    if (!documentId || !record) return { kind: 'new' }
    return this.#existing(documentId, record, requestHash)
  }

  /** Serialize the read/create transition inside the sole application writer. */
  async begin(ownerEmail, idempotencyKey, requestHash) {
    const run = async () => {
      const existing = await this.inspect(ownerEmail, idempotencyKey, requestHash)
      if (existing.kind !== 'new') return existing
      const now = Date.now()
      const domain = ownerEmail.toLowerCase().split('@')[1] || ''
      const record = /** @type {import('../repositories/send-requests.js').SendRequestRecord} */ ({
        id: `send-request-${randomUUID()}`,
        ownerEmail: ownerEmail.toLowerCase(), domain, idempotencyKey, requestHash,
        state: 'applying', resultJson: '', createdAt: now, updatedAt: now,
      })
      const documentId = await this.records.put(record)
      return { kind: 'started', documentId, record }
    }
    const operation = this._serial.then(run, run)
    this._serial = operation.then(() => undefined, () => undefined)
    return await operation
  }

  /** Commit the exact response before the route returns it to the browser. */
  async accept(documentId, result) {
    const resultJson = JSON.stringify(result)
    await this.records.update(documentId, { state: 'accepted', resultJson, updatedAt: Date.now() })
    return result
  }

  /** Remove a row only when every side effect was safely compensated. */
  async abort(documentId) { await this.records.delete(documentId) }

  /** Preserve uncertainty rather than allowing a duplicate delivery retry. */
  async markAmbiguous(documentId) {
    await this.records.update(documentId, { state: 'ambiguous', updatedAt: Date.now() })
  }

  #existing(documentId, record, requestHash) {
    if (record.requestHash !== requestHash) {
      throw new SendIdempotencyConflict('idempotencyKey was already used for a different message', 'mismatch')
    }
    if (record.state === 'accepted') {
      try { return { kind: 'replay', documentId, result: JSON.parse(record.resultJson) } } catch {
        throw new SendIdempotencyConflict('The accepted send result cannot be recovered safely', 'ambiguous')
      }
    }
    if (record.state === 'ambiguous') {
      throw new SendIdempotencyConflict('The previous send outcome is ambiguous and will not be retried automatically', 'ambiguous')
    }
    throw new SendIdempotencyConflict('The previous send is still being applied', 'applying')
  }
}

function canonicalJson(value) {
  if (value == null || typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`
  return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonicalJson(value[key])}`).join(',')}}`
}
