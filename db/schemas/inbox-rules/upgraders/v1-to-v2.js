/** Historical rules retain their explicit domain-wide behavior until edited. */
export default function upgrade(doc) { return { ...doc } }
