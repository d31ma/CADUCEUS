/** Historical HTML and undeclared fields never grant composer quote authority. */
export default function upgrade(doc) {
  return { ...doc, quoteProof: null }
}
