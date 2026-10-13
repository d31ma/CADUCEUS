/** Client idempotency metadata is optional for historical action records. */
export default function upgrade(doc) {
  return { ...doc }
}
