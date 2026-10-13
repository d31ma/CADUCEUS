/** Remove the retired phone-number identity field. */
export default function upgrade(doc) {
  const { phones: _phones, ...user } = doc
  return user
}
