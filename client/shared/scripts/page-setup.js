/**
 * Page-wide browser setup, linked into every page by `tac.scripts`.
 *
 * Escape hatch: this is the part of the old `imports.js` bootstrap that has
 * to run in the page before any service exists and that no Tac service can
 * own: in-memory storage fallbacks, the runtime configuration default, the
 * DuVay readiness promise that the UI tests wait on, the keyboard command
 * runtime, modal focus containment, the column resizers and the literal
 * interface localizer.
 * Markup and styles stay in the views and in the stylesheets of `tac.styles`.
 */

import './shortcuts.js'
import './keyboard.js'
import './dialog-focus.js'
import './column-resizers.js'

/** @returns {Storage} */
function memoryStorage() {
  const data = new Map()
  return {
    get length() { return data.size },
    key(index) { return Array.from(data.keys())[index] ?? null },
    getItem(key) { return data.has(String(key)) ? data.get(String(key)) : null },
    setItem(key, value) { data.set(String(key), String(value)) },
    removeItem(key) { data.delete(String(key)) },
    clear() { data.clear() },
  }
}

/** @param {'sessionStorage' | 'localStorage'} name */
function ensureStorage(name) {
  try {
    const storage = window[name]
    const probe = `__caduceus_probe_${Date.now()}`
    storage.setItem(probe, '1')
    storage.removeItem(probe)
  } catch {
    Object.defineProperty(window, name, { value: memoryStorage(), configurable: true })
  }
}

ensureStorage('sessionStorage')
ensureStorage('localStorage')

window.CADUCEUS_CONFIG ||= { apiUrl: '' }
window.CADUCEUS_CONFIG.brandLogo ||= '/shared/assets/favicon.svg'

/**
 * The literal interface localizer (and the generated catalogs it fetches)
 * matters only when the interface is not in English, so English pages do not
 * load it. PreferenceService sets lang and dir from Go either way.
 */
function interfaceLocale() {
  let choice = 'auto'
  try { choice = localStorage.getItem('caduceus-interface-locale-v1') || 'auto' } catch {}
  return (choice === 'auto' ? navigator.language || 'en' : choice).toLowerCase().split('-')[0]
}

if (interfaceLocale() !== 'en') {
  void Promise.all([import('./localization.js'), import('./interface-localizer.js')]).then(([localization, localizer]) => {
    localization.restoreInterfaceLocale()
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', localizer.installInterfaceLocalizer, { once: true })
    else localizer.installInterfaceLocalizer()
  })
}

/**
 * Resolves after DuVay has registered its custom elements and the browser has
 * painted twice. Page modules and browser tests wait on it.
 * @type {Promise<void>}
 */
window.__caduceusDuvayReady = customElements.whenDefined('w-btn').then(() => new Promise(resolve => {
  requestAnimationFrame(() => requestAnimationFrame(() => {
    document.documentElement.setAttribute('data-duvay-ready', '')
    document.dispatchEvent(new CustomEvent('caduceus:duvay-ready'))
    resolve()
  }))
}))
