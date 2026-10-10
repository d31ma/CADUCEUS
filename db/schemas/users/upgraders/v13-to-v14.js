/** Add an explicit, empty keyboard-command override map. */
export default function upgrade(doc) {
  return {
    ...doc,
    keyboardShortcutOverrides: {},
  }
}
