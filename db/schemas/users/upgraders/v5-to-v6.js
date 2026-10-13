const defaults = { 'new-mail': 'chime', 'hit-send': 'pop', send: 'pulse', confirm: 'bell' }
const sounds = new Set(['none', 'chime', 'bell', 'pop', 'pulse'])

/** Materialize the four local notification cue preferences. */
export default function upgrade(doc) {
  const existing = doc.notificationSounds && typeof doc.notificationSounds === 'object'
    ? doc.notificationSounds
    : {}
  return {
    ...doc,
    notificationSounds: Object.fromEntries(Object.entries(defaults).map(([event, fallback]) => [
      event,
      sounds.has(existing[event]) ? existing[event] : fallback,
    ])),
  }
}
