/** Reply/forward source identity is optional; historical rows remain valid. */
export default function upgrade(doc) {
  return { ...doc }
}
