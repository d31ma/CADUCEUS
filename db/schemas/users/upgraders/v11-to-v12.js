/** Add aggregate/focused-account unread badge preferences. */
export default function upgrade(doc) {
  const domains = Array.isArray(doc.domains) ? [...new Set(doc.domains)] : []
  return {
    ...doc,
    unreadBadgeScope: 'all',
    unreadBadgeDomain: domains[0] || undefined,
  }
}
