/**
 * CADUCEUS in-house keyboard shortcut manager.
 *
 * Replacement for the (former) `mousetrap` dependency. Exposes a global
 * `window._caduceusShortcuts` with a small `bind` / `unbind` / `reset` API
 * matching the subset of Mousetrap we actually used.
 *
 * Supported key-string grammar (matches keymap.json):
 *   - single character    "c", "j", "/", "?", "#"
 *   - named key           "enter", "esc", "tab", "space",
 *                         "up", "down", "left", "right",
 *                         "backspace", "delete"
 *   - modifier + key      "mod+enter", "shift+r", "alt+x", "ctrl+s"
 *                         (`mod` resolves to ⌘ on macOS / iOS, Ctrl elsewhere)
 *   - sequence            "g i"       (press g, then i within 1s)
 *                         "* a"
 *
 * Shifted symbol characters (`?`, `*`, `#`, `!`, etc.) are matched against
 * `event.key` directly — no need to write `shift+/` for `?`. Conversely,
 * `shift+r` only matches when Shift is held while pressing the letter r.
 *
 * Callback contract (matches Mousetrap):
 *   - return `false` → prevent default + stop propagation
 *   - return `true`  → pass-through (default browser behaviour)
 *   - return nothing → preventDefault is still called (Mousetrap's default)
 *
 * @module shortcuts
 */
;(function () {
  'use strict'

  /** @type {string} How `mod` is resolved on this platform. */
  var MOD = /Mac|iPhone|iPad/i.test(navigator.platform) ? 'meta' : 'ctrl'

  /** @type {number} How long (ms) we wait for the next key of a sequence. */
  var SEQUENCE_TIMEOUT_MS = 1000

  /** @type {Set<string>} Modifier names we recognise. */
  var MODIFIER_NAMES = new Set(['ctrl', 'control', 'alt', 'option', 'shift', 'meta', 'cmd', 'command', 'mod'])

  /** @type {Record<string, string>} Aliases → canonical modifier name. */
  var MODIFIER_ALIASES = {
    control: 'ctrl',
    option: 'alt',
    cmd: 'meta',
    command: 'meta',
  }

  /**
   * Map of named keys → the value `KeyboardEvent.key` produces for them.
   * Lets us write `"enter"` in the keymap and still match the literal
   * `"Enter"` (case-folded) on the event.
   *
   * @type {Object.<string, string>}
   */
  var NAMED_KEYS = {
    enter: 'enter',
    return: 'enter',
    esc: 'escape',
    escape: 'escape',
    tab: 'tab',
    space: ' ',
    spacebar: ' ',
    backspace: 'backspace',
    'delete': 'delete',
    del: 'delete',
    up: 'arrowup',
    down: 'arrowdown',
    left: 'arrowleft',
    right: 'arrowright',
  }

  /**
   * Canonicalise one chord segment ("shift+r", "mod+enter", "?", "g") into
   * a normalised string the matcher compares against.
   *
   * Result format: `[modifier...]+<key>` where modifiers are sorted
   * alphabetically and `<key>` is lowercase. Examples:
   *   "shift+R"     → "shift+r"
   *   "mod+enter"   → "ctrl+enter" (Linux/Windows) / "meta+enter" (macOS)
   *   "?"           → "?"
   *   "g"           → "g"
   *
   * @param {string} chord
   * @returns {string}
   */
  function canonicalChord(chord) {
    var parts = chord.split('+').map(function (p) { return p.trim().toLowerCase() })
    /** @type {Set<string>} */
    var mods = new Set()
    var key = ''
    for (var i = 0; i < parts.length; i++) {
      var p = parts[i]
      if (MODIFIER_NAMES.has(p)) {
        var canonical = MODIFIER_ALIASES[p] || p
        if (canonical === 'mod') canonical = MOD
        mods.add(canonical)
      } else {
        key = p
      }
    }
    if (Object.prototype.hasOwnProperty.call(NAMED_KEYS, key)) {
      key = NAMED_KEYS[key]
    }
    var sortedMods = Array.from(mods).sort()
    return sortedMods.length ? sortedMods.join('+') + '+' + key : key
  }

  /**
   * Parse a key-string like `"g i"` or `"mod+enter"` into an array of
   * canonical chords. A single chord → array length 1; sequences → multiple.
   *
   * @param {string} spec
   * @returns {string[]}
   */
  function parseKeySpec(spec) {
    return spec.split(/\s+/).filter(Boolean).map(canonicalChord)
  }

  /**
   * Build the canonical chord string for a live `KeyboardEvent`.
   *
   * Modifier keys themselves (Shift, Control, etc.) return `''` so the
   * matcher can ignore them; we only care about the chord *completed* when
   * the non-modifier key is pressed.
   *
   * Shifted symbol characters (`?`, `*`, `!`) are emitted as the symbol
   * itself rather than `shift+<base>`, mirroring Mousetrap's behaviour and
   * what users naturally write in keymaps.
   *
   * @param {KeyboardEvent} ev
   * @returns {string}
   */
  function chordFromEvent(ev) {
    var key = ev.key
    if (!key) return ''
    var lower = key.toLowerCase()
    // Ignore raw modifier keypresses — they only matter combined with a non-modifier.
    if (lower === 'shift' || lower === 'control' || lower === 'alt' || lower === 'meta') return ''

    /** @type {string[]} */
    var mods = []
    if (ev.ctrlKey) mods.push('ctrl')
    if (ev.altKey) mods.push('alt')
    if (ev.metaKey) mods.push('meta')

    // For single printable non-letter symbols (`?`, `*`, `#`, `/`...) the
    // user typed the symbol directly — don't tack on `shift+`. For letters,
    // the keymap convention is `shift+r`, so include shift then.
    var isLetter = /^[a-z]$/.test(lower)
    var includeShift = ev.shiftKey && (isLetter || NAMED_KEYS[lower] || lower === ' ' || lower.length > 1)
    if (includeShift) mods.push('shift')

    mods.sort()
    return mods.length ? mods.join('+') + '+' + lower : lower
  }

  /**
   * One registered binding.
   *
   * @typedef {object} Binding
   * @property {string} originalSpec
   * @property {string[]} chords        Canonicalised chord sequence
   * @property {(ev: KeyboardEvent) => (boolean | void)} callback
   */

  /** @type {Binding[]} */
  var bindings = []

  /**
   * In-progress sequence-matching state. We keep track of what subset of
   * registered chord arrays still match the keys pressed so far.
   *
   * @type {{ chords: string[], deadline: number } | null}
   */
  var sequenceState = null

  /**
   * Determine whether a given keyboard event should fire any binding.
   * Handles both single chords and multi-step sequences.
   *
   * @param {KeyboardEvent} ev
   * @returns {void}
   */
  function onKeyDown(ev) {
    var chord = chordFromEvent(ev)
    if (!chord) return

    // Expire stale sequence state.
    if (sequenceState && Date.now() > sequenceState.deadline) {
      sequenceState = null
    }

    var attemptedChords = sequenceState ? sequenceState.chords.concat(chord) : [chord]

    // Find bindings whose chord array starts with attemptedChords.
    /** @type {Binding[]} */
    var prefixMatches = []
    /** @type {Binding[]} */
    var exactMatches = []
    for (var i = 0; i < bindings.length; i++) {
      var b = bindings[i]
      if (b.chords.length < attemptedChords.length) continue
      var ok = true
      for (var j = 0; j < attemptedChords.length; j++) {
        if (b.chords[j] !== attemptedChords[j]) { ok = false; break }
      }
      if (!ok) continue
      if (b.chords.length === attemptedChords.length) exactMatches.push(b)
      else prefixMatches.push(b)
    }

    if (exactMatches.length === 0 && prefixMatches.length === 0) {
      // No match — abandon any in-progress sequence.
      sequenceState = null
      return
    }

    if (exactMatches.length > 0) {
      sequenceState = null
      // Fire every matching binding. Mousetrap convention: a `false`
      // return → preventDefault + stopPropagation. `true` → pass through.
      // No return → preventDefault (Mousetrap's default behaviour).
      var preventDefault = false
      for (var k = 0; k < exactMatches.length; k++) {
        var result = exactMatches[k].callback(ev)
        if (result !== true) preventDefault = true
      }
      if (preventDefault) {
        ev.preventDefault()
        ev.stopPropagation()
      }
      // If there are also longer-prefix matches, keep the sequence alive
      // for one more keystroke; otherwise we're done.
      if (prefixMatches.length === 0) return
    }

    // At least one binding has more chords to match — wait for the next key.
    sequenceState = { chords: attemptedChords, deadline: Date.now() + SEQUENCE_TIMEOUT_MS }
  }

  /**
   * Public API mirrored on `window._caduceusShortcuts`.
   *
   * @typedef {object} CaduceusShortcuts
   * @property {(spec: string, cb: (ev: KeyboardEvent) => boolean | void) => void} bind
   * @property {(spec: string) => void} unbind
   * @property {() => void} reset
   */

  /**
   * Register a callback for the given key-spec.
   *
   * @param {string} spec
   * @param {(ev: KeyboardEvent) => (boolean | void)} callback
   */
  function bind(spec, callback) {
    var chords = parseKeySpec(spec)
    if (chords.length === 0) return
    bindings.push({ originalSpec: spec, chords: chords, callback: callback })
  }

  /**
   * Remove every binding registered under `spec`.
   *
   * @param {string} spec
   */
  function unbind(spec) {
    bindings = bindings.filter(function (b) { return b.originalSpec !== spec })
  }

  /** Drop every registered binding and any in-flight sequence. */
  function reset() {
    bindings = []
    sequenceState = null
  }

  document.addEventListener('keydown', onKeyDown, true)

  /** @type {CaduceusShortcuts} */
  window._caduceusShortcuts = { bind: bind, unbind: unbind, reset: reset }
})()
