/** Label actions reuse the existing bounded JSON journal representation. */
export default function upgrade(doc) {
  return { ...doc }
}
