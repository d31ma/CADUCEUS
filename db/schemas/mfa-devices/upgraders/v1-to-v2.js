export default function upgrade(doc) {
  return { id: doc.id || crypto.randomUUID(), type: doc.type || 'totp', createdAt: doc.createdAt || new Date().toISOString(), ...doc }
}
