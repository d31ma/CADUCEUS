/** Add an exact-domain custom category catalog without inferring old data. */
export default function upgrade(doc) {
  return {
    ...doc,
    categoryCatalog: Object.fromEntries(
      (Array.isArray(doc.domains) ? doc.domains : []).map(domain => [domain, {}]),
    ),
  }
}
