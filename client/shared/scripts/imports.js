/**
 * CADUCEUS shell bootstrap — DuVay + PWA setup.
 *
 * Responsibilities:
 *   - Load the DuVay CSS + web-component bundle (self-hosted, vendored).
 *   - Initialize the colour-scheme theme from localStorage or OS preference.
 *   - Register the PWA service worker.
 *   - Chain-load the in-house shortcut binder + keyboard shortcut manager.
 *   - Provide a global toast / snackbar renderer.
 *
 * This file should be loaded synchronously in `<head>` so that the theme and
 * service worker start as early as possible.
 *
 * @module imports
 */

import { restoreThemePreferences } from './theme-preferences.js'
import { restoreInterfaceLocale } from './localization.js'
import { installInterfaceLocalizer } from './interface-localizer.js'

document.documentElement.lang = 'en'
document.documentElement.dir = 'ltr'

// ── Browser-owned runtime polish ────────────────────────────────────────

;(function installStorageFallbacks() {
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
      if (storage) {
        const probe = `__caduceus_probe_${Date.now()}`
        storage.setItem(probe, '1')
        storage.removeItem(probe)
        return
      }
    } catch {}

    try {
      Object.defineProperty(window, name, { value: memoryStorage(), configurable: true })
    } catch {
      window[name] = memoryStorage()
    }
  }

  ensureStorage('sessionStorage')
  ensureStorage('localStorage')
})()

// Locale is restored after storage fallbacks exist and before any Tac component
// mounts, preventing a left-to-right/English flash for returning users.
restoreInterfaceLocale()
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', installInterfaceLocalizer, { once: true })
} else {
  installInterfaceLocalizer()
}

;(function installSharedHeadAssets() {
  const headLinks = [
    ['manifest', '/shared/assets/manifest.webmanifest'],
    ['icon', '/shared/assets/favicon.svg', 'image/svg+xml'],
    ['apple-touch-icon', '/shared/assets/icon-192.png'],
    // DuVay first: defines the --w-* tokens that themes.css aliases onto --ms-*.
    ['stylesheet', '/shared/assets/duvay/duvay.min.css'],
    ['stylesheet', '/shared/assets/styles.css'],
    ['stylesheet', '/shared/assets/themes.css'],
  ]

  for (const [rel, href, type] of headLinks) {
    if (document.head.querySelector(`link[href="${href}"]`)) continue
    const link = document.createElement('link')
    link.rel = rel
    link.href = href
    if (type) link.type = type
    document.head.appendChild(link)
  }

  const meta = [
    ['theme-color', '#0d0d0d'],
    ['color-scheme', 'light'],
    ['mobile-web-app-capable', 'yes'],
    ['apple-mobile-web-app-capable', 'yes'],
    ['apple-mobile-web-app-status-bar-style', 'black-translucent'],
    ['apple-mobile-web-app-title', 'CADUCEUS'],
  ]

  for (const [name, content] of meta) {
    if (document.head.querySelector(`meta[name="${name}"]`)) continue
    const tag = document.createElement('meta')
    tag.name = name
    tag.content = content
    document.head.appendChild(tag)
  }

  window.CADUCEUS_CONFIG ||= { apiUrl: '' }
  window.CADUCEUS_CONFIG.brandLogo ||= '/shared/assets/favicon.svg'
})()

/**
 * Resolves after DuVay has registered its custom elements and the browser has
 * completed a paint. Page modules and browser tests use this instead of
 * guessing how long an asynchronous module upgrade might take.
 *
 * @type {Promise<void>}
 */
window.__caduceusDuvayReady = new Promise((resolve, reject) => {
  const src = '/shared/assets/duvay/duvay-wc.min.js'
  /** @type {HTMLScriptElement | null} */
  const existing = document.querySelector(`script[src="${src}"]`)
  /** @type {HTMLScriptElement} */
  const duvayScript = existing || document.createElement('script')

  const markReady = () => {
    requestAnimationFrame(() => requestAnimationFrame(() => {
      document.documentElement.setAttribute('data-duvay-ready', '')
      document.dispatchEvent(new CustomEvent('caduceus:duvay-ready'))
      resolve()
    }))
  }

  if (customElements.get('w-btn')) {
    markReady()
    return
  }

  duvayScript.addEventListener('load', markReady, { once: true })
  duvayScript.addEventListener('error', () => {
    reject(new Error(`Unable to load DuVay web components from ${src}`))
  }, { once: true })

  if (!existing) {
    duvayScript.type = 'module'
    duvayScript.src = src
    duvayScript.dataset.caduceusDuvay = 'true'
    document.head.appendChild(duvayScript)
  }
})

// ── Theme initialization ──────────────────────────────────────────────

// Restore the bounded catalog before app components mount. This also migrates
// the previous single `caduceus-theme` value without accepting CSS or URLs.
restoreThemePreferences()

// ── Keyboard shortcuts ────────────────────────────────────────────────

/**
 * Pull in the in-house shortcut binder, keyboard manager, and overflow-menu
 * helper as side-effect imports so the bundler folds them into `imports.js`.
 * Each script attaches to `window`; `keyboard.js` depends on the binding from
 * `shortcuts.js`, so order matters.
 */
import './shortcuts.js'
import './keyboard.js'
import './overflow-menu.js'
import './fetch-cache.js'
import './unread-badge.js'
import './dialog-focus.js'

if (window._caduceusKeyboard) {
  window._caduceusKeyboard.init()
}

// Global toasts — lightweight custom snackbar

/** @type {ReturnType<typeof setTimeout> | null} Auto-dismiss timeout ID */
let _toastTimer = null

/** @type {ReturnType<typeof setInterval> | null} Countdown interval ID */
let _toastCountdownTimer = null

/**
 * Display a toast / snackbar notification.
 *
 * Creates (or reuses) a fixed-position `<div id="caduceus-toast">` with
 * ARIA live-region attributes.  Supports two calling conventions:
 *
 * 1. **Simple string** — `_caduceusShowToast('Copied!', 3000)`
 * 2. **Interactive object** — `_caduceusShowToast({ msg: 'Undo?', duration: 5000, action: { label: 'Undo', onClick: fn } })`
 *
 * When an action button is provided the toast becomes interactive
 * (pointer-events enabled) and displays a countdown label.  Clicking the
 * action button dismisses the toast early and invokes the callback.
 *
 * @param {string|{
 *   msg: string,
 *   duration?: number,
 *   action?: { label: string, onClick: Function }
 * }} msgOrOpts - Toast message text or configuration object.
 * @param {number} [duration=2500] - Display duration in milliseconds (ignored when the first argument is an object that provides its own `duration`).
 */
window._caduceusShowToast = (msgOrOpts, duration = 2500) => {
  if (_toastTimer !== null) clearTimeout(_toastTimer)
  if (_toastCountdownTimer) if (_toastCountdownTimer !== null) clearInterval(_toastCountdownTimer)
  let el = document.getElementById('caduceus-toast')
  if (!el) {
    el = document.createElement('div')
    el.id = 'caduceus-toast'
    el.setAttribute('role', 'status')
    el.setAttribute('aria-live', 'polite')
    el.setAttribute('aria-atomic', 'true')
    el.style.cssText = 'position:fixed;bottom:1rem;left:50%;transform:translateX(-50%);z-index:9999;background:var(--ms-inverse-surface,#333);color:var(--ms-inverse-on-surface,#fff);padding:0.75rem 1.25rem;border-radius:var(--ms-shape-card,4px);font-size:14px;font-family:IBM Plex Sans,sans-serif;box-shadow:0 4px 12px rgba(0,0,0,0.3);transition:opacity 0.3s;opacity:0;pointer-events:none'
    document.body.appendChild(el)
  }
  // Support interactive toasts with action button
  if (typeof msgOrOpts === 'object' && msgOrOpts.msg) {
    const { msg, duration: dur, action } = msgOrOpts
    const d = dur ?? duration
    let seconds = Math.ceil(d / 1000)
    el.style.pointerEvents = 'auto'
    el.innerHTML = `<span id="caduceus-toast-msg">${msg} <span id="caduceus-toast-countdown">(${seconds}s)</span></span><button id="caduceus-toast-action" style="margin-left:1rem;background:var(--ms-primary,#1f6f8b);color:#fff;border:none;border-radius:var(--ms-shape-sharp,4px);padding:0.25rem 0.75rem;font-size:13px;cursor:pointer;font-family:inherit;font-weight:500;">${action?.label || ''}</button>`
    el.style.opacity = '1'

    // Countdown
    const countdownEl = document.getElementById('caduceus-toast-countdown')
    _toastCountdownTimer = setInterval(() => {
      seconds--
      if (seconds <= 0) {
        if (_toastCountdownTimer !== null) clearInterval(_toastCountdownTimer)
        el.style.opacity = '0'; el.style.pointerEvents = 'none'
      } else if (countdownEl) {
        countdownEl.textContent = `(${seconds}s)`
      }
    }, 1000)

    // Action button
    const actionBtn = document.getElementById('caduceus-toast-action')
    if (actionBtn && action?.onClick) {
      actionBtn.addEventListener('click', () => {
        if (_toastCountdownTimer !== null) clearInterval(_toastCountdownTimer)
        if (_toastTimer !== null) clearTimeout(_toastTimer)
        el.style.opacity = '0'; el.style.pointerEvents = 'none'
        action.onClick()
      })
    }

    _toastTimer = setTimeout(() => {
      if (_toastCountdownTimer !== null) clearInterval(_toastCountdownTimer)
      el.style.opacity = '0'; el.style.pointerEvents = 'none'
    }, d)
  } else {
    el.style.pointerEvents = 'none'
    el.innerHTML = ''
  el.textContent = typeof msgOrOpts === "string" ? msgOrOpts : msgOrOpts.msg
    el.style.opacity = '1'
    _toastTimer = setTimeout(() => { el.style.opacity = '0' }, duration)
  }
}
