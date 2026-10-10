// @ts-check

/**
 * @typedef {object} LoginFormProps
 * @property {string} [brandlogo] - Brand asset passed by the application shell
 */

/**
 * Multi-step authentication / login component.
 *
 * The client drives one authentication transaction and never decides anything
 * about it. Every step posts a factor and reads back the transaction's state;
 * the server says which factors this deployment requires and whether they have
 * been satisfied. `requiredFactors` comes from the server for exactly this
 * reason: the policy lives in one place, and it is not here.
 *
 * The flow progresses through these steps:
 * 1. `start` — address input, opens the transaction
 * 2. `password` — password factor, when the deployment requires it
 * 3. `passkey` — WebAuthn assertion
 * 4. `totp` — authenticator-app code
 * 5. `recovery` — single-use recovery code
 * 6. `password-setup` — set a password on a signed-in session
 * 7. `passkey-setup` — enrol a passkey on a signed-in session
 *
 * On success it hands the issued token set to the app shell.
 *
 * @prop {LoginFormProps} [props]
 */
export default class {
  /** @type {string} Current step in the login flow */
  $step = 'start'
  /** @type {string} */
  $email = ''; /** @type {string} */
  $totpCode = ''; /** @type {string} */
  $deviceCode = ''; /** @type {string} */
  $deviceName = ''
  /** @type {string} The authentication transaction every factor advances */
  $transactionId = ''
  /** @type {string[]} Factors this deployment requires, as the server reports them */
  $requiredFactors = []
  /** @type {boolean} Whether the transaction may now be completed */
  $satisfied = false
  /** @type {boolean} Whether this transaction has proved its primary password factor */
  $primarySatisfied = false
  /** @type {string} */
  $password = ''; /** @type {string} */
  $confirmPassword = ''; /** @type {string} */
  $recoveryCode = ''
  /** @type {'code' | 'admin'} */
  $recoveryMethod = 'code'
  /** @type {boolean} */
  $loading = false; /** @type {string} */
  $error = ''
  /**
   * @description Base API URL from the global CADUCEUS_CONFIG.
   * @returns {string}
   */
  get _api() { return window.CADUCEUS_CONFIG?.apiUrl || '' }

  /**
   * Read the current value of a form input by CSS selector.
   *
   * Safe to call server-side (returns empty string when `document` is undefined).
   *
   * @param {string} selector - CSS selector for the input element
   * @param {string} [fallback=''] - Fallback value when the element is not found
   * @returns {string} The trimmed input value, or the fallback
   */
  formValue(selector, fallback = '') {
    if (typeof document === 'undefined') return String(fallback ?? '').trim()
    const host = document.querySelector(selector)
    const control = host?.matches('input, textarea, select')
      ? host
      : host?.querySelector('input, textarea, select')
    const value = /** @type {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null | undefined} */ (control)?.value
      ?? /** @type {{ value?: unknown } | null} */ (host)?.value
    return String(value ?? fallback ?? '').trim()
  }

  /**
   * POST one JSON body and return `{ ok, data }`.
   *
   * @param {string} path
   * @param {Record<string, unknown>} body
   * @param {string} [bearer]
   * @returns {Promise<{ ok: boolean, data: any }>}
   */
  async post(path, body, bearer) {
    const headers = /** @type {Record<string, string>} */ ({ 'Content-Type': 'application/json' })
    if (bearer) headers.Authorization = `Bearer ${bearer}`
    const response = await fetch(`${this._api}${path}`, {
      method: 'POST', headers, body: JSON.stringify(body),
    })
    return { ok: response.ok, data: await response.json() }
  }

  /** @param {ArrayBuffer} value @returns {string} */
  bufferToBase64Url(value) {
    const bytes = new Uint8Array(value)
    let raw = ''
    for (const byte of bytes) raw += String.fromCharCode(byte)
    return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }

  /** @param {string} value @returns {ArrayBuffer} */
  base64UrlToBuffer(value) {
    const padded = value.replace(/-/g, '+').replace(/_/g, '/')
    const raw = atob(padded + '='.repeat((4 - (padded.length % 4)) % 4))
    const bytes = new Uint8Array(raw.length)
    for (let index = 0; index < raw.length; index += 1) bytes[index] = raw.charCodeAt(index)
    return bytes.buffer
  }

  // ── Transaction state ────────────────────────────────────────────────────

  /**
   * Adopt one reported transaction state and move to the step that satisfies
   * whatever it still needs. When the server says the transaction is satisfied
   * the session is issued here and nowhere else.
   *
   * @param {any} state
   * @returns {Promise<void>}
   */
  async adopt(state) {
    this.$transactionId = state.transactionId ?? this.$transactionId
    this.$requiredFactors = state.requiredFactors ?? this.$requiredFactors
    this.$satisfied = Boolean(state.satisfied)
    if (state.state === 'failed') {
      this.$error = 'Unable to sign in with those details.'
      this.$step = 'start'
      return
    }
    if (this.$satisfied) {
      await this.complete()
      return
    }
    if (this.$requiredFactors.includes('password') && this.$step === 'start') {
      this.$step = 'password'
      return
    }
    this.$step = 'passkey'
    await this.authenticatePasskey()
  }

  /**
   * Exchange the satisfied transaction for the session and its tokens.
   *
   * @returns {Promise<void>}
   */
  async complete() {
    const { ok, data } = await this.post('/auth/login/complete', {
      transactionId: this.$transactionId,
    })
    if (!ok) { this.$error = data.error || 'Unable to complete sign-in.'; return }
    this.$transactionId = ''
    // @ts-ignore — the app shell owns token storage and navigation.
    window._caduceus?.handleLogin(data)
  }

  // ── Step: start ──────────────────────────────────────────────────────────

  /**
   * Begin the login: submit the address and open a transaction.
   *
   * The response never distinguishes an unknown address from a known one, so
   * this step reveals nothing about which mailboxes exist.
   *
   * @returns {Promise<void>}
   */
  async requestMfa() {
    this.$email = this.formValue('[data-login-email]', this.$email)
    if (!this.$email) { this.$error = 'Email is required.'; return }
    this.$loading = true; this.$error = ''
    try {
      const { ok, data } = await this.post('/auth/login/begin', { address: this.$email })
      if (!ok) { this.$error = data.error || 'Unable to sign in.'; return }
      this.$primarySatisfied = false
      await this.adopt(data)
    } catch { this.$error = 'Network error. Check your connection.' }
    finally { this.$loading = false }
  }

  // ── Step: password ───────────────────────────────────────────────────────

  /** @returns {Promise<void>} */
  async verifyPassword() {
    this.$password = this.formValue('[data-login-password]', this.$password)
    if (!this.$transactionId) { this.$error = 'Start again from your email address.'; return }
    if (!this.$password) { this.$error = 'Password is required.'; return }
    this.$loading = true; this.$error = ''
    try {
      const { ok, data } = await this.post('/auth/login/password', {
        transactionId: this.$transactionId, password: this.$password,
      })
      this.$password = ''
      if (!ok) { this.$error = data.error || 'Unable to verify password.'; return }
      this.$primarySatisfied = true
      this.$step = 'passkey'
      await this.adopt(data)
    } catch { this.$error = 'Network error. Check your connection.' }
    finally { this.$loading = false }
  }

  // ── Step: passkey ────────────────────────────────────────────────────────

  /**
   * Answer the passkey challenge for the in-flight transaction.
   *
   * @returns {Promise<void>}
   */
  async authenticatePasskey() {
    if (typeof PublicKeyCredential === 'undefined') {
      this.$error = this.$primarySatisfied
        ? 'Passkeys are not supported on this device. Use a recovery code.'
        : 'Passkeys are not supported on this device. Enter your password to continue.'
      this.$step = this.$primarySatisfied ? 'recovery' : 'password'
      return
    }
    try {
      this.$loading = true
      const { ok, data } = await this.post('/auth/login/passkey/options', {
        transactionId: this.$transactionId,
      })
      if (!ok) { this.$error = data.error || 'Unable to start passkey sign-in.'; return }
      if (!data.credentialIds?.length) {
        this.$error = this.$primarySatisfied
          ? 'No passkey is enrolled yet. Use a recovery code to finish signing in.'
          : 'No passkey is enrolled yet. Enter your password to continue.'
        this.$step = this.$primarySatisfied ? 'recovery' : 'password'
        return
      }
      const assertion = /** @type {PublicKeyCredential | null} */ (
        await navigator.credentials.get({
          publicKey: {
            challenge: this.base64UrlToBuffer(data.challenge),
            rpId: data.relyingPartyId,
            userVerification: 'required',
            allowCredentials: data.credentialIds.map((/** @type {string} */ id) => ({
              id: this.base64UrlToBuffer(id), type: 'public-key',
            })),
          },
        })
      )
      if (!assertion) { this.$error = 'No passkey credential was returned.'; return }
      const response = /** @type {AuthenticatorAssertionResponse} */ (assertion.response)
      const verified = await this.post('/auth/login/passkey', {
        transactionId: this.$transactionId,
        credentialId: assertion.id,
        authenticatorData: this.bufferToBase64Url(response.authenticatorData),
        clientDataJSON: this.bufferToBase64Url(response.clientDataJSON),
        signature: this.bufferToBase64Url(response.signature),
      })
      if (!verified.ok) { this.$error = verified.data.error || 'Authentication failed.'; return }
      await this.adopt(verified.data)
    } catch (error) {
      const name = /** @type {Error} */ (error).name
      this.$error = name === 'NotAllowedError' || name === 'AbortError'
        ? 'Passkey authentication was cancelled.'
        : 'Passkey error: ' + (error instanceof Error ? error.message : 'Unknown')
      this.$step = 'passkey'
    }
    finally { this.$loading = false }
  }

  // ── Step: TOTP ───────────────────────────────────────────────────────────

  /** @returns {Promise<void>} */
  async confirmTotp() {
    this.$totpCode = this.formValue('[data-login-totp]', this.$totpCode)
    if (!/^\d{6}$/.test(this.$totpCode)) {
      this.$error = 'Enter the 6-digit code from your authenticator app.'
      return
    }
    this.$loading = true; this.$error = ''
    try {
      const { ok, data } = await this.post('/auth/login/totp', {
        transactionId: this.$transactionId, code: this.$totpCode,
      })
      this.$totpCode = ''
      if (!ok) { this.$error = data.error || 'Invalid code.'; return }
      await this.adopt(data)
    } catch { this.$error = 'Network error. Check your connection.' }
    finally { this.$loading = false }
  }

  // ── Step: recovery ───────────────────────────────────────────────────────

  /** @returns {void} */
  useRecoveryCode() { this.$recoveryMethod = 'code'; this.$recoveryCode = ''; this.$error = ''; this.$step = 'recovery' }

  /** @returns {void} */
  useAdminRecovery() { this.$recoveryMethod = 'admin'; this.$recoveryCode = ''; this.$error = ''; this.$step = 'recovery' }

  /** @returns {void} */
  toggleRecoveryMethod() {
    if (this.$recoveryMethod === 'code') this.useAdminRecovery()
    else this.useRecoveryCode()
  }

  /**
   * Spend a single-use recovery code as the second factor.
   *
   * A recovery code is a factor in the same transaction as any other, so it
   * signs the user in rather than granting a separate restoration mode. The
   * credential they came for is then enrolled on an ordinary session.
   *
   * @returns {Promise<void>}
   */
  async verifyRecoveryCode() {
    this.$recoveryCode = this.formValue('[data-login-recovery-code]', this.$recoveryCode)
    if (!this.$transactionId) { this.$error = 'Start again from your email address.'; return }
    if (!this.$recoveryCode) { this.$error = 'A recovery code is required.'; return }
    this.$loading = true; this.$error = ''
    try {
      const { ok, data } = await this.post('/auth/login/recovery', {
        transactionId: this.$transactionId, code: this.$recoveryCode,
      })
      this.$recoveryCode = ''
      if (!ok) { this.$error = data.error || 'Invalid recovery code.'; return }
      await this.adopt(data)
    } catch { this.$error = 'Network error. Check your connection.' }
    finally { this.$loading = false }
  }

  /**
   * An administrator-issued code is spent the same way: it is one more
   * single-use code against the same transaction.
   *
   * @returns {Promise<void>}
   */
  async verifyAdminRecovery() {
    this.$email = this.formValue('[data-login-email]', this.$email)
    this.$recoveryCode = this.formValue('[data-login-admin-code]', this.$recoveryCode)
    if (!this.$transactionId) {
      await this.requestMfa()
      if (!this.$transactionId) return
    }
    await this.verifyRecoveryCode()
  }

  // ── Signed-in credential setup ───────────────────────────────────────────

  /**
   * Set a password on a signed-in session.
   *
   * Changing a credential needs a freshly proven factor, so this opens a second
   * transaction and satisfies it before the change is submitted.
   *
   * @returns {Promise<void>}
   */
  async setupPassword() {
    this.$password = this.formValue('[data-login-new-password]', this.$password)
    this.$confirmPassword = this.formValue('[data-login-confirm-password]', this.$confirmPassword)
    if (!this.$password) { this.$error = 'Password is required.'; return }
    if (this.$confirmPassword && this.$confirmPassword !== this.$password) {
      this.$error = 'Passwords do not match.'
      return
    }
    this.$loading = true; this.$error = ''
    try {
      const stepUp = await this.stepUp()
      if (!stepUp) return
      const response = await fetch(`${this._api}/auth/credentials/password`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${this.token()}` },
        body: JSON.stringify({ stepUpTransactionId: stepUp, password: this.$password }),
      })
      const data = await response.json()
      this.$password = ''
      this.$confirmPassword = ''
      if (!response.ok) { this.$error = data.error || 'Unable to save password.'; return }
      this.$step = 'passkey-setup'
      await this.registerPasskey()
    } catch { this.$error = 'Network error. Check your connection.' }
    finally { this.$loading = false }
  }

  /**
   * Enrol a passkey on a signed-in session.
   *
   * @returns {Promise<void>}
   */
  async registerPasskey() {
    if (typeof PublicKeyCredential === 'undefined') {
      this.$error = 'Passkeys are not supported on this device.'
      this.$step = 'start'
      return
    }
    try {
      this.$loading = true
      const stepUp = await this.stepUp()
      if (!stepUp) return
      const begun = await this.post(
        '/auth/credentials/passkey/begin', { stepUpTransactionId: stepUp }, this.token(),
      )
      if (!begun.ok) { this.$error = begun.data.error || 'Registration failed.'; return }
      const credential = /** @type {PublicKeyCredential | null} */ (
        await navigator.credentials.create({
          publicKey: {
            challenge: this.base64UrlToBuffer(begun.data.challenge),
            rp: { id: begun.data.relyingPartyId, name: 'CADUCEUS' },
            user: {
              id: this.base64UrlToBuffer(begun.data.principalId.replace(/[^A-Za-z0-9_-]/g, '')),
              name: this.$email,
              displayName: this.$email,
            },
            pubKeyCredParams: [{ type: 'public-key', alg: -7 }, { type: 'public-key', alg: -257 }],
            authenticatorSelection: { userVerification: 'required', residentKey: 'preferred' },
          },
        })
      )
      if (!credential) { this.$error = 'No passkey credential was returned.'; return }
      const response = /** @type {AuthenticatorAttestationResponse} */ (credential.response)
      const saved = await this.post('/auth/credentials/passkey/finish', {
        attestationObject: this.bufferToBase64Url(response.attestationObject),
        clientDataJSON: this.bufferToBase64Url(response.clientDataJSON),
      }, this.token())
      if (!saved.ok) { this.$error = saved.data.error || 'Failed to save passkey.'; return }
      this.$error = ''
      this.$step = 'start'
    } catch (error) {
      const name = /** @type {Error} */ (error).name
      this.$error = name === 'NotAllowedError' || name === 'AbortError'
        ? 'Passkey registration was cancelled.'
        : 'Passkey error: ' + (error instanceof Error ? error.message : 'Unknown')
    }
    finally { this.$loading = false }
  }

  /**
   * Prove a factor again for a credential change, and return the transaction
   * that proves it.
   *
   * @returns {Promise<string>} the satisfied transaction, or '' on failure
   */
  async stepUp() {
    const begun = await this.post('/auth/login/begin', { address: this.$email })
    if (!begun.ok) { this.$error = begun.data.error || 'Unable to re-authenticate.'; return '' }
    const transactionId = begun.data.transactionId
    const code = this.formValue('[data-login-recovery-code]', this.$recoveryCode)
    if (!code) {
      this.$error = 'Confirm it is you: enter a recovery code to change credentials.'
      this.$step = 'recovery'
      return ''
    }
    const verified = await this.post('/auth/login/recovery', { transactionId, code })
    if (!verified.ok || !verified.data.satisfied) {
      this.$error = verified.data.error || 'That code did not confirm your identity.'
      return ''
    }
    return transactionId
  }

  /** @returns {string} the stored access token */
  token() {
    if (typeof sessionStorage === 'undefined') return ''
    return sessionStorage.getItem('caduceus_token') ?? ''
  }

  /** @returns {void} */
  usePassword() { this.$error = ''; this.$step = 'password' }

  /**
   * Navigate back one step in the login flow. Clears the current error.
   *
   * @returns {void}
   */
  back() {
    this.$error = ''
    if (this.$step === 'passkey') { this.$step = 'start' }
    else if (this.$step === 'totp') { this.$totpCode = ''; this.$step = 'start' }
    else if (this.$step === 'password') { this.$password = ''; this.$step = 'start' }
    else if (this.$step === 'recovery') { this.$recoveryCode = ''; this.$step = 'start' }
  }
}
