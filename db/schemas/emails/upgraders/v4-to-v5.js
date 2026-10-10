/** Existing messages start with no independent provider labels. */
export default function upgrade(doc) {
  return { ...doc, labels: Array.isArray(doc.labels) ? doc.labels : [] }
}
