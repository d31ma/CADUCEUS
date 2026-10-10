/**
 * Tracking and exact-owner fields are optional. Historical rows remain valid
 * and use the explicit legacy owner fallback in the activity repository.
 */
export default function upgrade(doc) {
  return { ...doc }
}
