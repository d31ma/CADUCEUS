/**
 * Context-aware, user-customizable keyboard command runtime.
 *
 * Only one action is selected for a keypress: the rendered route/component
 * context is resolved first, then one command event is emitted. This prevents
 * shared keys such as Shift+R from broadcasting unrelated actions.
 *
 * @module keyboard
 */

;(function () {
  'use strict'

  var INPUT_SELECTORS = 'input, textarea, select, [contenteditable="true"]'
  var VALID_SCOPES = new Set(['global', 'mailbox', 'mail', 'reader', 'reader-find', 'reader-no-find', 'composer', 'calendar'])
  var VALID_MODIFIERS = new Set(['alt', 'ctrl', 'meta', 'mod', 'shift'])
  var MODIFIER_ALIASES = { control: 'ctrl', option: 'alt', cmd: 'meta', command: 'meta' }
  var NAMED_KEYS = new Set(['backspace', 'delete', 'down', 'enter', 'esc', 'left', 'right', 'space', 'tab', 'up'])
  var MAX_BINDING_LENGTH = 64
  var MAX_SEQUENCE_LENGTH = 2

  /** @type {Array<Record<string, any>>} */
  var catalog = []
  /** @type {Record<string, string>} */
  var overrides = {}
  /** @type {Record<string, string>} */
  var bindings = {}
  var initialized = false
  /** @type {Promise<void> | null} */
  var initializing = null

  function isDev() {
    return location.hostname === 'localhost' || location.hostname === '127.0.0.1' || new URLSearchParams(location.search).has('dev')
  }

  function isVisible(selector) {
    var element = document.querySelector(selector)
    if (!(element instanceof HTMLElement)) return false
    var style = getComputedStyle(element)
    return style.display !== 'none' && style.visibility !== 'hidden' && !element.hidden && element.getClientRects().length > 0
  }

  /** Return the one effective command context for the current rendered view. */
  function currentContext() {
    if (isVisible('[data-shortcut-help-dialog]')) return 'dialog'
    if (isVisible('.compose-panel') || /^\/compose\/?$/.test(location.pathname)) return 'composer'
    if (isVisible('.calendar-page') || /^\/calendar(?:\/|\/index\.html)?$/.test(location.pathname)) return 'calendar'
    if (isVisible('.email-detail-shell')) return isVisible('.find-in-thread') ? 'reader-find' : 'reader'
    if (/^\/(inbox|drafts|sent|archive|spam|trash|scheduled|folder)(\/|$)/.test(location.pathname)) return 'mailbox'
    return 'other'
  }

  function contextsForScope(scope) {
    if (scope === 'global') return ['other', 'mailbox', 'reader', 'reader-find', 'composer', 'calendar']
    if (scope === 'mail') return ['mailbox', 'reader', 'reader-find']
    if (scope === 'reader') return ['reader', 'reader-find']
    if (scope === 'reader-find') return ['reader-find']
    if (scope === 'reader-no-find') return ['reader']
    return [scope]
  }

  function isActionAvailable(action, context) {
    if (context === 'dialog') return action.id === 'core:show-shortcuts'
    return contextsForScope(action.scope).indexOf(context) !== -1
  }

  function isInputFocused() {
    var element = document.activeElement
    if (!element) return false
    try {
      if (element.matches(INPUT_SELECTORS)) return true
      if (element.shadowRoot && element.shadowRoot.activeElement) return true
    } catch (_) {}
    return !!element.closest(INPUT_SELECTORS)
  }

  /** Normalize a portable key specification without resolving `mod`. */
  function normalizeBinding(value) {
    if (typeof value !== 'string') return null
    var source = value.trim().toLowerCase().replace(/\s*\+\s*/g, '+').replace(/\s+/g, ' ')
    if (!source || source.length > MAX_BINDING_LENGTH) return null
    if (source === 'off') return 'off'
    var sequence = source.split(' ')
    if (sequence.length > MAX_SEQUENCE_LENGTH) return null
    var normalized = []
    for (var index = 0; index < sequence.length; index++) {
      var pieces = sequence[index].split('+').map(function (piece) { return piece.trim() })
      if (pieces.some(function (piece) { return !piece })) return null
      var modifiers = []
      var key = ''
      for (var partIndex = 0; partIndex < pieces.length; partIndex++) {
        var piece = MODIFIER_ALIASES[pieces[partIndex]] || pieces[partIndex]
        if (VALID_MODIFIERS.has(piece)) {
          if (modifiers.indexOf(piece) !== -1) return null
          modifiers.push(piece)
        } else {
          if (key) return null
          key = piece === 'escape' ? 'esc' : piece
        }
      }
      if (!key || (!NAMED_KEYS.has(key) && !/^[a-z0-9?!*#/,._-]$/.test(key))) return null
      modifiers.sort()
      normalized.push((modifiers.length ? modifiers.join('+') + '+' : '') + key)
    }
    return normalized.join(' ')
  }

  function bindingConflicts(nextBindings) {
    var conflicts = []
    for (var leftIndex = 0; leftIndex < catalog.length; leftIndex++) {
      var left = catalog[leftIndex]
      var leftBinding = nextBindings[left.id]
      if (!leftBinding || leftBinding === 'off') continue
      for (var rightIndex = leftIndex + 1; rightIndex < catalog.length; rightIndex++) {
        var right = catalog[rightIndex]
        if (nextBindings[right.id] !== leftBinding) continue
        var leftContexts = contextsForScope(left.scope)
        var rightContexts = contextsForScope(right.scope)
        if (leftContexts.some(function (context) { return rightContexts.indexOf(context) !== -1 })) {
          conflicts.push({ binding: leftBinding, actionIds: [left.id, right.id], labels: [left.label, right.label] })
        }
      }
    }
    return conflicts
  }

  function validateOverrides(value) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { ok: false, error: 'Keyboard shortcut overrides must be an object.' }
    var allowed = new Map(catalog.map(function (action) { return [action.id, action] }))
    var normalizedOverrides = {}
    for (var entry of Object.entries(value)) {
      var actionId = entry[0]
      var action = allowed.get(actionId)
      if (!action) return { ok: false, error: 'Unknown keyboard command: ' + actionId }
      var normalized = normalizeBinding(entry[1] === '' ? 'off' : entry[1])
      if (!normalized) return { ok: false, error: 'Invalid shortcut for ' + action.label + '.' }
      if (normalized !== action.binding) normalizedOverrides[actionId] = normalized
    }
    var nextBindings = Object.fromEntries(catalog.map(function (action) {
      return [action.id, normalizedOverrides[action.id] || action.binding]
    }))
    var conflicts = bindingConflicts(nextBindings)
    if (conflicts.length) {
      var first = conflicts[0]
      return { ok: false, error: first.labels.join(' and ') + ' both use ' + displayBinding(first.binding) + '.', conflicts: conflicts }
    }
    return { ok: true, overrides: normalizedOverrides, bindings: nextBindings, conflicts: [] }
  }

  function displayBinding(binding) {
    if (!binding || binding === 'off') return 'Disabled'
    var isMac = /Mac|iPhone|iPad/i.test(navigator.platform)
    return binding.split(' ').map(function (chord) {
      return chord.split('+').map(function (part) {
        if (part === 'mod') return isMac ? '⌘' : 'Ctrl'
        if (part === 'meta') return isMac ? '⌘' : 'Meta'
        if (part === 'ctrl') return 'Ctrl'
        if (part === 'alt') return isMac ? '⌥' : 'Alt'
        if (part === 'shift') return isMac ? '⇧' : 'Shift'
        if (part === 'up') return '↑'
        if (part === 'down') return '↓'
        if (part === 'left') return '←'
        if (part === 'right') return '→'
        if (part === 'esc') return 'Esc'
        if (part === 'enter') return 'Enter'
        return part.length === 1 ? part.toUpperCase() : part
      }).join(isMac ? '' : '+')
    }).join(' then ')
  }

  function dispatch(action) {
    window.dispatchEvent(new CustomEvent('caduceus:shortcut:' + action.id, {
      detail: { actionId: action.id, binding: bindings[action.id], context: currentContext() },
    }))
    if (isDev()) console.log('[keyboard] ' + action.id + ' (' + bindings[action.id] + ')')
  }

  function bindAll() {
    if (!window._caduceusShortcuts) return
    window._caduceusShortcuts.reset()
    var byBinding = new Map()
    for (var index = 0; index < catalog.length; index++) {
      var action = catalog[index]
      var binding = bindings[action.id]
      if (!binding || binding === 'off') continue
      if (!byBinding.has(binding)) byBinding.set(binding, [])
      byBinding.get(binding).push(action)
    }
    for (var item of byBinding.entries()) {
      ;(function (binding, actions) {
        window._caduceusShortcuts.bind(binding, function (event) {
          var context = currentContext()
          var action = actions.find(function (candidate) { return isActionAvailable(candidate, context) })
          if (!action) return true
          if (isInputFocused() && action.allowInInput !== true) return true
          event.preventDefault()
          dispatch(action)
          return false
        })
      })(item[0], item[1])
    }
  }

  async function init() {
    if (initialized) return
    if (initializing) return initializing
    initializing = (async function () {
      var response = await fetch('/shared/data/keymap.json', { cache: 'no-cache' })
      if (!response.ok) throw new Error('Keyboard command catalog could not be loaded.')
      var payload = await response.json()
      if (payload?.schemaVersion !== 1 || !Array.isArray(payload.actions)) throw new Error('Keyboard command catalog is invalid.')
      var ids = new Set()
      catalog = payload.actions.map(function (entry) {
        if (!entry || typeof entry.id !== 'string' || ids.has(entry.id) || !VALID_SCOPES.has(entry.scope)) throw new Error('Keyboard command catalog is invalid.')
        var binding = normalizeBinding(entry.binding)
        if (!binding || binding === 'off') throw new Error('Keyboard command catalog contains an invalid default.')
        ids.add(entry.id)
        return { id: entry.id, label: String(entry.label || entry.id), group: String(entry.group || 'Other'), scope: entry.scope, binding: binding, allowInInput: entry.allowInInput === true }
      })
      var result = validateOverrides(overrides)
      if (!result.ok) throw new Error(result.error)
      bindings = result.bindings || {}
      overrides = result.overrides || {}
      bindAll()
      initialized = true
    })().finally(function () { initializing = null })
    return initializing
  }

  async function applyOverrides(value) {
    await init()
    var result = validateOverrides(value || {})
    if (!result.ok) return result
    overrides = result.overrides || {}
    bindings = result.bindings || {}
    bindAll()
    window.dispatchEvent(new CustomEvent('caduceus:keyboard-shortcuts-changed', { detail: getState() }))
    return { ok: true, overrides: { ...overrides }, bindings: { ...bindings }, conflicts: [] }
  }

  function getCatalog() { return catalog.map(function (action) { return { ...action } }) }
  function getBindings() { return { ...bindings } }
  function getOverrides() { return { ...overrides } }
  function getGroups() {
    var groups = []
    for (var action of catalog) {
      var group = groups.find(function (candidate) { return candidate.label === action.group })
      if (!group) { group = { label: action.group, rows: [] }; groups.push(group) }
      group.rows.push({ ...action, binding: bindings[action.id], displayBinding: displayBinding(bindings[action.id]) })
    }
    return groups
  }
  function getState() { return { catalog: getCatalog(), bindings: getBindings(), overrides: getOverrides(), groups: getGroups() } }
  function destroy() {
    window._caduceusShortcuts?.reset()
    initialized = false
    initializing = null
    catalog = []
    bindings = {}
    overrides = {}
  }

  window._caduceusKeyboard = {
    init: init,
    ready: init,
    destroy: destroy,
    applyOverrides: applyOverrides,
    validateOverrides: validateOverrides,
    normalizeBinding: normalizeBinding,
    displayBinding: displayBinding,
    getCatalog: getCatalog,
    getBindings: getBindings,
    getOverrides: getOverrides,
    getGroups: getGroups,
    getContext: currentContext,
  }
})()
