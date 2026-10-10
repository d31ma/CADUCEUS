export default function upgrade(doc) { return { id: doc.id || crypto.randomUUID(), ...doc } }
