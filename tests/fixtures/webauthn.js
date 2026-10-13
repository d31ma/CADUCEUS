/**
 * Synthetic WebAuthn ceremony material.
 *
 * The service ships a CBOR *decoder* only — real attestation objects come from
 * an authenticator — so tests need a small encoder plus an ES256 key to stand
 * in for one.
 */
import { createHash, generateKeyPairSync, createSign } from 'node:crypto'

const head = (major, value) => {
  if (value < 24) return Buffer.from([(major << 5) | value])
  if (value < 0x100) return Buffer.from([(major << 5) | 24, value])
  if (value < 0x10000) {
    const b = Buffer.alloc(3)
    b[0] = (major << 5) | 25
    b.writeUInt16BE(value, 1)
    return b
  }
  const b = Buffer.alloc(5)
  b[0] = (major << 5) | 26
  b.writeUInt32BE(value, 1)
  return b
}

export const cbor = value => {
  if (Buffer.isBuffer(value)) return Buffer.concat([head(2, value.length), value])
  if (typeof value === 'string') {
    const bytes = Buffer.from(value, 'utf8')
    return Buffer.concat([head(3, bytes.length), bytes])
  }
  if (typeof value === 'number') return value < 0 ? head(1, -1 - value) : head(0, value)
  if (typeof value === 'boolean') return Buffer.from([value ? 0xf5 : 0xf4])
  if (value === null) return Buffer.from([0xf6])
  if (Array.isArray(value)) return Buffer.concat([head(4, value.length), ...value.map(cbor)])
  const entries = value instanceof Map ? [...value] : Object.entries(value)
  return Buffer.concat([head(5, entries.length), ...entries.flatMap(([k, v]) => [cbor(k), cbor(v)])])
}

export const FLAG = { present: 0x01, verified: 0x04, multiDevice: 0x08, backedUp: 0x10, attested: 0x40 }
export const b64u = buf => Buffer.from(buf).toString('base64url')

/** A stand-in authenticator holding one ES256 credential. */
export class FakeAuthenticator {
  /** @param {{ rpId?: string, origin?: string, credentialId?: string }} [options] */
  constructor({ rpId = 'localhost', origin = 'http://localhost:8080', credentialId = 'test-credential' } = {}) {
    this.rpId = rpId
    this.origin = origin
    this.credentialId = Buffer.from(credentialId)
    this.keys = generateKeyPairSync('ec', { namedCurve: 'prime256v1' })
  }

  /** The credential id as WebAuthn transmits it. */
  get id() {
    return this.credentialId.toString('base64url')
  }

  /** COSE_Key (ES256) for the credential public key. */
  coseKey() {
    const jwk = this.keys.publicKey.export({ format: 'jwk' })
    return Buffer.from(cbor(new Map([
      [1, 2], [3, -7], [-1, 1],
      [-2, Buffer.from(jwk.x, 'base64url')],
      [-3, Buffer.from(jwk.y, 'base64url')],
    ])))
  }

  /** @param {{ flags?: number, signCount?: number, attested?: boolean }} [options] */
  authData({ flags = FLAG.present | FLAG.verified, signCount = 1, attested = false } = {}) {
    const counter = Buffer.alloc(4)
    counter.writeUInt32BE(signCount)
    const prefix = Buffer.concat([
      createHash('sha256').update(this.rpId).digest(),
      Buffer.from([attested ? flags | FLAG.attested : flags]),
      counter,
    ])
    if (!attested) return prefix
    const len = Buffer.alloc(2)
    len.writeUInt16BE(this.credentialId.length)
    return Buffer.concat([prefix, Buffer.alloc(16), len, this.credentialId, this.coseKey()])
  }

  clientData(type, challenge) {
    return Buffer.from(JSON.stringify({ type, challenge, origin: this.origin }), 'utf8')
  }

  /** A `navigator.credentials.create()` response for the given challenge. */
  register(challenge, options = {}) {
    return {
      id: this.id,
      response: {
        clientDataJSON: b64u(this.clientData('webauthn.create', challenge)),
        attestationObject: b64u(cbor({
          fmt: 'none',
          attStmt: {},
          authData: this.authData({ ...options, attested: true }),
        })),
      },
    }
  }

  /** A `navigator.credentials.get()` response for the given challenge. */
  authenticate(challenge, options = {}) {
    const clientDataJSON = this.clientData('webauthn.get', challenge)
    const data = this.authData(options)
    const signer = createSign('sha256')
    signer.update(Buffer.concat([data, createHash('sha256').update(clientDataJSON).digest()]))
    return {
      id: this.id,
      response: {
        clientDataJSON: b64u(clientDataJSON),
        authenticatorData: b64u(data),
        signature: b64u(signer.sign(this.keys.privateKey)),
      },
    }
  }
}
