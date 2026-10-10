/**
 * The browser half of connecting an outside mailbox: it reads the OAuth
 * redirect and the "?onboarding=accounts" request from the address, removes
 * the provider's secrets from browser history, leaves for the provider's
 * sign-in page, and keeps the onboarding flags other pages read. A
 * WebAssembly service cannot read or change the address or these storage keys.
 */
@Service
export default class ProviderAuthorizationService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;
  active = true

  static _PENDING = 'caduceus_account_onboarding_pending'
  static _PROVIDER = 'caduceus_oauth_provider'
  static _CALLBACK = ['code', 'state', 'error']
  static _ONBOARDING = 'onboarding'
  static _YES = '1'

  @OnStart
  start() {
    const url = new URL(location.href)
    const params = url.searchParams
    const [codeName, stateName, errorName] = ProviderAuthorizationService._CALLBACK
    const code = [params.get(codeName)].join('')
    const state = [params.get(stateName)].join('')
    /** @type {ProviderLaunch} */
    const launch = {
      onboarding: [params.get(ProviderAuthorizationService._ONBOARDING) === 'accounts', ProviderAuthorizationService._read(localStorage, ProviderAuthorizationService._PENDING) === ProviderAuthorizationService._YES].includes(true),
      hasCallback: ProviderAuthorizationService._CALLBACK.map(name => params.has(name)).includes(true),
      failed: [params.has(errorName), code === '', state === ''].includes(true),
      code, state, provider: ProviderAuthorizationService._provider(state),
    }
    ProviderAuthorizationService._clean(url)
    this.applicationEventPublisher.publishEvent(launch, true)
  }

  /** @param {ProviderRedirectRequested} requested */
  @OnEvent
  redirect(requested) {
    ProviderAuthorizationService._write(sessionStorage, ProviderAuthorizationService._PROVIDER, requested.provider)
    ProviderAuthorizationService._write(localStorage, `${ProviderAuthorizationService._PROVIDER}:${requested.state}`, requested.provider)
    ProviderAuthorizationService._pending(requested.onboarding)
    location.assign(requested.url)
  }

  /** @param {AccountOnboardingChanged} changed */
  @OnEvent
  onboarding(changed) {
    ProviderAuthorizationService._pending(changed.pending)
    ProviderAuthorizationService._completed(changed.completed)
  }

  /** The provider remembered for this redirect's state, then for this tab. @param {string} state */
  static _provider(state) {
    const remembered = ProviderAuthorizationService._read(localStorage, `${ProviderAuthorizationService._PROVIDER}:${state}`)
    const session = ProviderAuthorizationService._read(sessionStorage, ProviderAuthorizationService._PROVIDER)
    ProviderAuthorizationService._remove(localStorage, `${ProviderAuthorizationService._PROVIDER}:${state}`)
    ProviderAuthorizationService._remove(sessionStorage, ProviderAuthorizationService._PROVIDER)
    return [remembered, session].filter(name => ['gmail', 'microsoft'].includes(name)).concat('')[0]
  }

  /** @param {URL} url */
  static _clean(url) {
    const names = [...ProviderAuthorizationService._CALLBACK, 'error_description', 'error_uri', 'session_state', ProviderAuthorizationService._ONBOARDING]
    for (const name of names) url.searchParams.delete(name)
    history.replaceState(history.state, '', `${url.pathname}${url.search}${url.hash}`)
  }

  /** @param {boolean} pending */
  static _pending(pending) {
    if (pending) ProviderAuthorizationService._write(localStorage, ProviderAuthorizationService._PENDING, ProviderAuthorizationService._YES)
  }

  /** @param {boolean} completed */
  static _completed(completed) {
    if (!completed) return
    const owner = [ProviderAuthorizationService._read(sessionStorage, 'caduceus_email'), 'owner'].filter(Boolean)[0]
    ProviderAuthorizationService._remove(localStorage, ProviderAuthorizationService._PENDING)
    ProviderAuthorizationService._write(localStorage, `caduceus_account_onboarding_complete:${owner}`, ProviderAuthorizationService._YES)
  }

  /** @param {Storage} store @param {string} key */
  static _read(store, key) {
    try { return [store.getItem(key)].join('') } catch { return '' }
  }

  /** @param {Storage} store @param {string} key @param {string} value */
  static _write(store, key, value) {
    try { store.setItem(key, value) } catch {}
  }

  /** @param {Storage} store @param {string} key */
  static _remove(store, key) {
    try { store.removeItem(key) } catch {}
  }
}
