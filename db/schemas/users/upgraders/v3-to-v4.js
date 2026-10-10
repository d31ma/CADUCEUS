/** Materialize privacy-safe remote-image preference defaults. */
export default function upgrade(doc) {
  return {
    ...doc,
    remoteImagePolicy: doc.remoteImagePolicy === 'allow' ? 'allow' : 'block',
    trustedImageSenders: Array.isArray(doc.trustedImageSenders) ? doc.trustedImageSenders : [],
  }
}
