/** Add a server-validated, account-synchronized interface locale choice. */
export default function upgrade(doc) {
  return { ...doc, interfaceLocale: 'auto' }
}
