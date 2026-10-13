/** Preserve legacy subject keys until a new RFC-threaded reminder is stored. */
export default function upgrade(doc) {
  return { ...doc }
}
