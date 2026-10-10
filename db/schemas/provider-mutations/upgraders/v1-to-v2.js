/** Folder lifecycle fields are optional; historical message mutations remain unchanged. */
export default function upgrade(doc) {
  return { ...doc }
}
