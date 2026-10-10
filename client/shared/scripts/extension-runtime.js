// @ts-check

const MAX_RESULT_BYTES = 16 * 1024
const MAX_FIELD_LENGTH = 32 * 1024
const COMMAND_TIMEOUT_MS = 2_000

const THEME_VARIABLES = Object.freeze({
  danger: '--ms-error',
  'on-primary': '--ms-on-primary',
  primary: '--ms-primary',
  'primary-container': '--ms-primary-container',
  surface: '--ms-surface',
  'surface-container': '--ms-surface-container',
  'surface-container-high': '--ms-surface-container-high',
  text: '--ms-on-surface',
  'text-subtle': '--ms-on-surface-variant',
})
const COLOR = /^#[0-9a-fA-F]{3,8}$/
const CONTEXT_FIELDS = Object.freeze({
  mail: ['id', 'subject', 'sender', 'receivedAt'],
  compose: ['to', 'cc', 'bcc', 'subject', 'text'],
})
const COMPOSE_PATCH_FIELDS = Object.freeze(['to', 'cc', 'bcc', 'subject', 'text'])

/** Convert a validated declarative theme manifest into fixed local CSS variables. */
export function extensionThemeDeclarations(manifest) {
  const tokens = plainObject(manifest?.theme?.tokens) ? manifest.theme.tokens : {}
  const declarations = {}
  for (const [token, variable] of Object.entries(THEME_VARIABLES)) {
    const value = tokens[token]
    if (typeof value === 'string' && COLOR.test(value)) declarations[variable] = value.toLowerCase()
  }
  return declarations
}

/** Permission-filter the host context before it crosses the worker boundary. */
export function filterExtensionContext(permissions, input) {
  const granted = new Set(Array.isArray(permissions) ? permissions : [])
  const result = { slot: boundedString(input?.slot, 64) }
  if (granted.has('mail:metadata')) result.mail = pickStrings(input?.mail, CONTEXT_FIELDS.mail)
  if (granted.has('compose:read')) result.compose = pickStrings(input?.compose, CONTEXT_FIELDS.compose)
  return result
}

/** Permission-filter and bound an untrusted worker result before applying it to the page. */
export function filterExtensionResult(permissions, input) {
  if (!plainObject(input)) return {}
  const granted = new Set(Array.isArray(permissions) ? permissions : [])
  const result = {}
  if (granted.has('ui:notify') && typeof input.notify === 'string' && input.notify.trim()) {
    result.notify = boundedString(input.notify.trim(), 200)
  }
  if (granted.has('compose:write') && plainObject(input.composePatch)) {
    const patch = pickStrings(input.composePatch, COMPOSE_PATCH_FIELDS)
    if (Object.keys(patch).length > 0) result.composePatch = patch
  }
  return JSON.stringify(result).length <= MAX_RESULT_BYTES ? result : {}
}

/**
 * Build an opaque-origin iframe document. The iframe's only script creates a
 * classic Web Worker from an in-memory Blob; package source never executes in
 * the application document or the iframe's main thread.
 */
export function extensionSandboxDocument(extension, sessionNonce) {
  const extensionId = boundedString(extension?.id, 128)
  const nonce = boundedString(sessionNonce, 128)
  const source = typeof extension?.source === 'string' ? extension.source : ''
  const workerSource = `"use strict";
let commandHandler = null;
const caduceus = Object.freeze({
  activate(candidate) {
    if (!candidate || typeof candidate.onCommand !== "function") throw new Error("Extension must register onCommand");
    if (commandHandler) throw new Error("Extension may activate only once");
    commandHandler = candidate.onCommand;
  }
});
globalThis.caduceus = caduceus;
self.onmessage = async event => {
  const message = event.data || {};
  if (message.type !== "command" || !commandHandler) return;
  try {
    const result = await commandHandler(Object.freeze(message.command), Object.freeze(message.context));
    self.postMessage({ type: "result", requestId: message.requestId, result });
  } catch (error) {
    self.postMessage({ type: "error", requestId: message.requestId, error: String(error && error.message || error) });
  }
};
try {
${source}
  if (!commandHandler) throw new Error("Extension did not activate");
  self.postMessage({ type: "ready" });
} catch (error) {
  self.postMessage({ type: "error", error: String(error && error.message || error) });
}`
  const encodedWorker = base64Encode(workerSource)
  const bootstrap = `
const extensionId = ${JSON.stringify(extensionId)};
const sessionNonce = ${JSON.stringify(nonce)};
const workerBytes = Uint8Array.from(atob(${JSON.stringify(encodedWorker)}), character => character.charCodeAt(0));
const worker = new Worker(URL.createObjectURL(new Blob([workerBytes], { type: "text/javascript" })));
const pending = new Map();
const send = message => parent.postMessage({ channel: "caduceus-extension", extensionId, sessionNonce, ...message }, "*");
const startupTimeout = setTimeout(() => { worker.terminate(); send({ type: "error", error: "Extension activation timed out" }); }, ${COMMAND_TIMEOUT_MS});
worker.onmessage = event => {
  const message = event.data || {};
  let serialized = "";
  try { serialized = JSON.stringify(message); } catch { worker.terminate(); send({ type: "error", error: "Extension returned an invalid result" }); return; }
  if (serialized.length > ${MAX_RESULT_BYTES}) { worker.terminate(); send({ type: "error", error: "Extension result exceeded 16 KiB" }); return; }
  if (message.type === "ready") clearTimeout(startupTimeout);
  if (message.requestId) { clearTimeout(pending.get(message.requestId)); pending.delete(message.requestId); }
  send(message);
};
worker.onerror = () => { clearTimeout(startupTimeout); worker.terminate(); send({ type: "error", error: "Extension worker failed" }); };
addEventListener("message", event => {
  const message = event.data || {};
  if (event.source !== parent || message.channel !== "caduceus-extension-host" || message.sessionNonce !== sessionNonce || message.extensionId !== extensionId || message.type !== "command") return;
  const timeout = setTimeout(() => {
    worker.terminate(); pending.delete(message.requestId);
    send({ type: "error", requestId: message.requestId, error: "Extension command timed out" });
  }, ${COMMAND_TIMEOUT_MS});
  pending.set(message.requestId, timeout);
  worker.postMessage({ type: "command", requestId: message.requestId, command: message.command, context: message.context });
});`
  return `<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-${htmlAttribute(nonce)}'; worker-src blob:; connect-src 'none'; img-src 'none'; style-src 'none'; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'"><script nonce="${htmlAttribute(nonce)}">${bootstrap}<\/script>`
}

class ExtensionRuntime {
  constructor() {
    this.packages = []
    this.sandboxes = new Map()
    this.requests = new Map()
    this.themePrevious = new Map()
    this.observer = null
    this.messageListener = event => this.receive(event)
    this.reloadListener = () => { void this.load() }
  }

  async load() {
    this.clear()
    const apiFetch = globalThis.window?._caduceus?.apiFetch
    if (!apiFetch || !globalThis.window?._caduceus?.auth?.isLoggedIn) return
    const response = await apiFetch('/extensions')
    if (!response?.ok) return
    const packages = await response.json()
    if (!Array.isArray(packages)) return
    const enabled = packages.filter(candidate => candidate?.enabled === true)
    const theme = enabled.find(candidate => candidate?.kind === 'theme')
    if (theme) this.applyTheme(theme.manifest)
    for (const projection of enabled.filter(candidate => candidate?.kind === 'plugin')) {
      const runtimeResponse = await apiFetch(`/extensions/${encodeURIComponent(projection.id)}`)
      if (!runtimeResponse?.ok) continue
      const extension = await runtimeResponse.json()
      if (extension?.enabled === true && typeof extension?.source === 'string') this.activate(extension)
    }
    this.mountContributions()
    if (typeof MutationObserver === 'function' && document.body) {
      this.observer = new MutationObserver(() => this.mountContributions())
      this.observer.observe(document.body, { childList: true, subtree: true })
    }
  }

  clear() {
    this.observer?.disconnect()
    this.observer = null
    for (const sandbox of this.sandboxes.values()) {
      clearTimeout(sandbox.readyTimeout)
      sandbox.resolveReady(false)
      sandbox.iframe.remove()
    }
    this.sandboxes.clear()
    for (const request of this.requests.values()) {
      clearTimeout(request.timeout)
      request.reject(new Error('Extension runtime was stopped'))
    }
    this.requests.clear()
    this.packages = []
    document.querySelectorAll('[data-caduceus-extension-command]').forEach(node => node.remove())
    document.getElementById('caduceus-extension-theme')?.remove()
    for (const [name, value] of this.themePrevious) {
      if (value) document.documentElement.style.setProperty(name, value)
      else document.documentElement.style.removeProperty(name)
    }
    this.themePrevious.clear()
  }

  applyTheme(manifest) {
    const declarations = extensionThemeDeclarations(manifest)
    if (Object.keys(declarations).length === 0) return
    const style = document.createElement('style')
    style.id = 'caduceus-extension-theme'
    style.dataset.caduceusExtensionTheme = 'true'
    style.textContent = `:root{${Object.entries(declarations).map(([name, value]) => `${name}:${value}`).join(';')}}`
    document.head.append(style)
    // Built-in accents intentionally use inline variables for immediate
    // theme switching. Save those exact values and let the explicitly enabled
    // package override them until disable/removal/sign-out.
    for (const [name, value] of Object.entries(declarations)) {
      this.themePrevious.set(name, document.documentElement.style.getPropertyValue(name))
      document.documentElement.style.setProperty(name, value)
    }
  }

  activate(extension) {
    const manifest = extension?.manifest
    if (!plainObject(manifest) || !Array.isArray(manifest.contributions) || !Array.isArray(manifest.permissions)) return
    const nonce = randomNonce()
    const iframe = document.createElement('iframe')
    iframe.hidden = true
    iframe.tabIndex = -1
    iframe.title = `${boundedString(extension.displayName || extension.id, 80)} extension sandbox`
    iframe.setAttribute('sandbox', 'allow-scripts')
    iframe.setAttribute('referrerpolicy', 'no-referrer')
    iframe.srcdoc = extensionSandboxDocument(extension, nonce)
    document.body.append(iframe)
    let resolveReady
    const readyPromise = new Promise(resolve => { resolveReady = resolve })
    const readyTimeout = setTimeout(() => resolveReady(false), COMMAND_TIMEOUT_MS + 1_000)
    this.packages.push(extension)
    this.sandboxes.set(extension.id, { iframe, nonce, ready: false, readyPromise, resolveReady, readyTimeout })
  }

  mountContributions() {
    for (const extension of this.packages) {
      for (const contribution of extension.manifest.contributions) {
        for (const slot of document.querySelectorAll(`[data-extension-slot="${contribution.slot}"]`)) {
          const exists = [...slot.children].some(child => {
            const element = /** @type {HTMLElement} */ (child)
            return element.dataset.extensionId === extension.id && element.dataset.extensionCommand === contribution.id
          })
          if (exists) continue
          const button = document.createElement('button')
          button.type = 'button'
          button.className = 'caduceus-extension-command'
          button.dataset.caduceusExtensionCommand = 'true'
          button.dataset.extensionId = extension.id
          button.dataset.extensionCommand = contribution.id
          button.textContent = contribution.label
          button.title = `${extension.displayName}: ${contribution.label}`
          button.disabled = !this.sandboxes.get(extension.id)?.ready
          button.addEventListener('click', () => { void this.invoke(extension, contribution, slot) })
          slot.append(button)
        }
      }
    }
  }

  async invoke(extension, contribution, slot) {
    const sandbox = this.sandboxes.get(extension.id)
    if (!sandbox?.iframe.contentWindow) return
    if (!await sandbox.readyPromise) {
      window._caduceus?.toast('Extension failed to start.')
      return
    }
    const requestId = randomNonce()
    const context = filterExtensionContext(extension.manifest.permissions, contextFromSlot(slot, contribution.slot))
    const resultPromise = new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.requests.delete(requestId)
        reject(new Error('Extension command did not respond'))
      }, COMMAND_TIMEOUT_MS + 1_000)
      this.requests.set(requestId, { resolve, reject, timeout, extensionId: extension.id, source: sandbox.iframe.contentWindow })
    })
    sandbox.iframe.contentWindow.postMessage({
      channel: 'caduceus-extension-host', type: 'command', extensionId: extension.id,
      sessionNonce: sandbox.nonce, requestId, command: { id: contribution.id, slot: contribution.slot }, context,
    }, '*')
    try {
      const raw = await resultPromise
      const result = filterExtensionResult(extension.manifest.permissions, raw)
      if (result.notify) window._caduceus?.toast(result.notify)
      if (result.composePatch) applyComposePatch(slot, result.composePatch)
      void (/** @type {any} */ (globalThis)).__tc_rerender?.()
    } catch (error) {
      window._caduceus?.toast(`Extension failed: ${String(/** @type {any} */ (error)?.message || error)}`)
      void (/** @type {any} */ (globalThis)).__tc_rerender?.()
    }
  }

  receive(event) {
    const message = event.data
    if (!plainObject(message) || message.channel !== 'caduceus-extension' || typeof message.extensionId !== 'string') return
    const sandbox = this.sandboxes.get(message.extensionId)
    if (!sandbox || event.source !== sandbox.iframe.contentWindow || message.sessionNonce !== sandbox.nonce) return
    if (message.type === 'ready') {
      clearTimeout(sandbox.readyTimeout)
      sandbox.ready = true
      sandbox.resolveReady(true)
      document.querySelectorAll('[data-caduceus-extension-command]').forEach(node => {
        const button = /** @type {HTMLButtonElement} */ (node)
        if (button.dataset.extensionId === message.extensionId) button.disabled = false
      })
      return
    }
    if (!message.requestId) {
      if (message.type === 'error') { clearTimeout(sandbox.readyTimeout); sandbox.resolveReady(false) }
      return
    }
    const request = this.requests.get(message.requestId)
    if (!request || request.extensionId !== message.extensionId || request.source !== event.source) return
    clearTimeout(request.timeout)
    this.requests.delete(message.requestId)
    if (message.type === 'result') request.resolve(message.result)
    else request.reject(new Error(boundedString(message.error || 'Extension failed', 200)))
  }

  bind() {
    window.addEventListener('message', this.messageListener)
    window.addEventListener('caduceus:extensions-reload', this.reloadListener)
  }
}

/** @type {ExtensionRuntime | null} */
let runtime = null

export async function loadTrustedExtensions() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  if (!runtime) { runtime = new ExtensionRuntime(); runtime.bind() }
  await runtime.load()
}

export function clearTrustedExtensions() {
  runtime?.clear()
}

function contextFromSlot(slot, name) {
  const result = { slot: name }
  if (name === 'message-toolbar') {
    result.mail = {
      id: slot.dataset.messageId || '',
      subject: slot.dataset.messageSubject || '',
      sender: slot.dataset.messageSender || '',
      receivedAt: slot.dataset.messageReceivedAt || '',
    }
  }
  if (name === 'composer-toolbar') {
    const panel = slot.closest('.compose-panel')
    result.compose = {
      to: fieldValue(panel?.querySelector('[data-compose-to]')),
      cc: fieldValue(panel?.querySelector('[data-compose-cc]')),
      bcc: fieldValue(panel?.querySelector('[data-compose-bcc]')),
      subject: fieldValue(panel?.querySelector('[data-compose-subject]')),
      text: boundedString(panel?.querySelector('[data-compose-editor]')?.textContent || '', MAX_FIELD_LENGTH),
    }
  }
  return result
}

function applyComposePatch(slot, patch) {
  const panel = slot.closest('.compose-panel')
  if (!panel) return
  for (const name of ['to', 'cc', 'bcc', 'subject']) {
    if (!(name in patch)) continue
    const field = panel.querySelector(`[data-compose-${name}]`)
    if (!field) continue
    field.value = patch[name]
    field.setAttribute('value', patch[name])
    field.dispatchEvent(new Event('input', { bubbles: true, composed: true }))
  }
  if ('text' in patch) {
    const editor = panel.querySelector('[data-compose-editor]')
    if (editor) {
      editor.textContent = patch.text
      editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: null }))
    }
  }
}

function pickStrings(value, fields) {
  const result = {}
  if (!plainObject(value)) return result
  for (const field of fields) if (typeof value[field] === 'string') result[field] = boundedString(value[field], MAX_FIELD_LENGTH)
  return result
}

function fieldValue(field) {
  return boundedString(typeof field?.value === 'string' ? field.value : field?.getAttribute?.('value') || '', MAX_FIELD_LENGTH)
}

function boundedString(value, maximum) { return String(value ?? '').slice(0, maximum) }
function plainObject(value) { return !!value && typeof value === 'object' && !Array.isArray(value) }
function htmlAttribute(value) { return String(value).replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;') }
function randomNonce() {
  const bytes = new Uint8Array(18)
  globalThis.crypto.getRandomValues(bytes)
  return [...bytes].map(value => value.toString(16).padStart(2, '0')).join('')
}
function base64Encode(value) {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
}
