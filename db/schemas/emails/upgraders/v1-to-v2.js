/**
 * Historical rows predate retained RFC headers. Keep those fields absent so
 * the thread repository can apply its explicit legacy-subject fallback rather
 * than fabricating relationships that cannot be proven.
 */
export default function upgrade(doc) {
  return { ...doc }
}
