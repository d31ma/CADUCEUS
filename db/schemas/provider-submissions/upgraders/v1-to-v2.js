export default function upgrade(document) {
  return { ...document, revision: 1 }
}
