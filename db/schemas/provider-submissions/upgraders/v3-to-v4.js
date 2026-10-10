export default function upgrade(document) {
  return {
    ...document,
    replacementEnvelopeFrom: '',
    replacementRecipients: [],
    replacementRawBucketIds: [],
    replacementDigests: [],
    replacementNotBefore: '',
  }
}
