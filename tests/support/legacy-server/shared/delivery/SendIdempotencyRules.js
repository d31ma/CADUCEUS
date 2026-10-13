import { createHash, randomUUID } from 'node:crypto'

export class SendIdempotencyRules {
  static canonicalJson(value) {
    if (value == null || typeof value !== 'object') return JSON.stringify(value)
    if (Array.isArray(value)) return `[${value.map(SendIdempotencyRules.canonicalJson).join(',')}]`
    return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${SendIdempotencyRules.canonicalJson(value[key])}`).join(',')}}`
  }
}
