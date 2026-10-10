// @ts-check
import { randomBytes } from 'node:crypto'

/** RFC 9562 identifiers that work under Node as well as Bun. */
export class Uuid {
  /**
   * A version 7 UUID: 48 bits of Unix milliseconds, then random bits, so
   * identifiers sort by creation time.
   *
   * @returns {string}
   */
  static v7() {
    const bytes = randomBytes(16)
    const now = BigInt(Date.now())
    for (let index = 0; index < 6; index++) bytes[index] = Number((now >> BigInt(8 * (5 - index))) & 0xffn)
    bytes[6] = (bytes[6] & 0x0f) | 0x70
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = bytes.toString('hex')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }
}
