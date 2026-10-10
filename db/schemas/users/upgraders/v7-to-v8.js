/** Add the privacy-gated public GitHub profile lookup preference, off by default. */
export default function upgrade(doc) {
  return {
    ...doc,
    githubProfileEnrichment: doc.githubProfileEnrichment === true,
  }
}
