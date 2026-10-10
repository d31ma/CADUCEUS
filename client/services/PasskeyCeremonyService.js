/**
 * Runs the browser's WebAuthn ceremony that creates a passkey. Tachyon has
 * no WebAuthn API for WebAssembly services, so PasskeysService asks for the
 * ceremony with PasskeyCeremonyRequested and hears PasskeyCeremonyFinished.
 */
@Service
export default class PasskeyCeremonyService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;
  active = true

  /** @param {PasskeyCeremonyRequested} requested */
  @OnEvent
  async create(requested) {
    /** @type {PasskeyCeremonyFinished} */
    const finished = await PasskeyCeremonyService._ceremony(requested).catch(PasskeyCeremonyService._failure)
    this.applicationEventPublisher.publishEvent(finished, false)
  }

  /** @param {PasskeyCeremonyRequested} requested @returns {Promise<PasskeyCeremonyFinished>} */
  static async _ceremony(requested) {
    if (typeof PublicKeyCredential === 'undefined') return { attestationObject: '', clientDataJSON: '', problem: 'Passkeys are not supported on this device.' }
    const credential = /** @type {PublicKeyCredential} */ (await navigator.credentials.create({
      publicKey: {
        challenge: PasskeyCeremonyService._buffer(requested.challenge),
        rp: { id: requested.relyingPartyId, name: 'CADUCEUS' },
        user: { id: PasskeyCeremonyService._buffer(requested.userId), name: requested.name, displayName: requested.displayName },
        pubKeyCredParams: [-7, -257].map(alg => ({ type: /** @type {'public-key'} */ ('public-key'), alg })),
        authenticatorSelection: { userVerification: 'required', residentKey: 'preferred' },
      },
    }))
    const attestation = /** @type {AuthenticatorAttestationResponse} */ (credential.response)
    return {
      attestationObject: PasskeyCeremonyService._text(attestation.attestationObject),
      clientDataJSON: PasskeyCeremonyService._text(attestation.clientDataJSON),
      problem: '',
    }
  }

  /** @param {unknown} error @returns {PasskeyCeremonyFinished} */
  static _failure(error) {
    const cancelled = /** @type {Error} */ (error)?.name === 'NotAllowedError'
    return { attestationObject: '', clientDataJSON: '', problem: cancelled ? 'Passkey creation was cancelled.' : 'Unable to add passkey.' }
  }

  /** @param {string} value @returns {ArrayBuffer} */
  static _buffer(value) {
    const padded = value.replace(/-/g, '+').replace(/_/g, '/')
    const quantum = 4
    const raw = atob(padded + '='.repeat((quantum - (padded.length % quantum)) % quantum))
    return Uint8Array.from(raw, character => character.charCodeAt(0)).buffer
  }

  /** @param {ArrayBuffer} value */
  static _text(value) {
    return btoa(String.fromCharCode(...new Uint8Array(value))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }
}
