/**
 * Legacy records keep their storagePath until their content is rewritten into
 * the attachment-files bucket. New records carry bucketId instead.
 *
 * @param {Record<string, unknown>} doc
 * @returns {Record<string, unknown>}
 */
export default function upgrade(doc) {
  return { ...doc }
}
