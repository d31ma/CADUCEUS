/**
 * Escape hatch: WebAuthn ceremonies for the login form.
 *
 * `navigator.credentials` has no Tachyon browser API, so LoginService (Go)
 * publishes PasskeyRequested and this service answers with PasskeyAnswered,
 * carrying the authenticator output as base64url text.
 */
@Service
export default class PasskeyService {
  static _REGISTRATION = 'create'
  static _PUBLIC_KEY = 'public-key'
  static _VERIFICATION = 'required'
  static _NO_CREDENTIAL = 'error:No passkey credential was returned.'
  static _CANCELLATIONS = ['NotAllowedError', 'AbortError']
  static _BASE64_BLOCK = 4

  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher

  /** @param {PasskeyRequested} request */
  @OnEvent
  async ceremony(request) {
    /** @type {PasskeyAnswered} */
    const answer = await this._attempt(request)
    this.applicationEventPublisher.publishEvent(answer, false)
  }

  /** @param {PasskeyRequested} request @returns {Promise<PasskeyAnswered>} */
  _attempt(request) {
    return this._ceremony(request.mode)(request)
      .catch(error => this._failed(request.mode, this._reason(this._cancelled(error), this._detail(error))))
  }

  /** @param {string} mode */
  _ceremony(mode) {
    return mode === PasskeyService._REGISTRATION ? this._create.bind(this) : this._get.bind(this)
  }

  /** @param {PasskeyRequested} request @returns {Promise<PasskeyAnswered>} */
  async _get(request) {
    const assertion = /** @type {PublicKeyCredential | null} */ (await navigator.credentials.get({
      publicKey: {
        challenge: this._buffer(request.challenge),
        rpId: request.relyingPartyId,
        userVerification: PasskeyService._VERIFICATION,
        allowCredentials: request.credentials.map(id => ({ id: this._buffer(id), type: PasskeyService._PUBLIC_KEY })),
      },
    }))
    if (!assertion) return this._failed(request.mode, PasskeyService._NO_CREDENTIAL)
    const response = /** @type {AuthenticatorAssertionResponse} */ (assertion.response)
    return {
      ...this._failed(request.mode, ''),
      credentialId: assertion.id,
      authenticatorData: this._text(response.authenticatorData),
      clientDataJSON: this._text(response.clientDataJSON),
      signature: this._text(response.signature),
    }
  }

  /** @param {PasskeyRequested} request @returns {Promise<PasskeyAnswered>} */
  async _create(request) {
    const credential = /** @type {PublicKeyCredential | null} */ (await navigator.credentials.create({
      publicKey: {
        challenge: this._buffer(request.challenge),
        rp: { id: request.relyingPartyId, name: 'CADUCEUS' },
        user: { id: this._buffer(request.userId.replace(/[^A-Za-z0-9_-]/g, '')), name: request.userName, displayName: request.userName },
        pubKeyCredParams: [{ type: PasskeyService._PUBLIC_KEY, alg: -7 }, { type: PasskeyService._PUBLIC_KEY, alg: -257 }],
        authenticatorSelection: { userVerification: PasskeyService._VERIFICATION, residentKey: 'preferred' },
      },
    }))
    if (!credential) return this._failed(request.mode, PasskeyService._NO_CREDENTIAL)
    const response = /** @type {AuthenticatorAttestationResponse} */ (credential.response)
    return {
      ...this._failed(request.mode, ''),
      attestationObject: this._text(response.attestationObject),
      clientDataJSON: this._text(response.clientDataJSON),
    }
  }

  /** @param {unknown} error */
  _cancelled(error) {
    return PasskeyService._CANCELLATIONS.includes(String(/** @type {Error} */ (error)?.name))
  }

  /** @param {unknown} error */
  _detail(error) {
    return error instanceof Error ? error.message : 'Unknown'
  }

  /** @param {boolean} cancelled @param {string} detail */
  _reason(cancelled, detail) {
    return cancelled ? 'cancelled' : `error:${detail}`
  }

  /** @param {string} mode @param {string} failure @returns {PasskeyAnswered} */
  _failed(mode, failure) {
    return { mode, failure, credentialId: '', authenticatorData: '', clientDataJSON: '', signature: '', attestationObject: '' }
  }

  /** @param {ArrayBuffer} value */
  _text(value) {
    const raw = Array.from(new Uint8Array(value), byte => String.fromCharCode(byte)).join('')
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }

  /** @param {string} value */
  _buffer(value) {
    const block = PasskeyService._BASE64_BLOCK
    const padded = value.replace(/-/g, '+').replace(/_/g, '/')
    const raw = atob(padded + '='.repeat((block - (padded.length % block)) % block))
    return Uint8Array.from(raw, character => character.charCodeAt(0)).buffer
  }
}
