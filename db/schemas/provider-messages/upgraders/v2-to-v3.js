/** Gmail canonical identities and multi-folder membership are optional. */
export default function upgrade(doc) {
  return { ...doc }
}
