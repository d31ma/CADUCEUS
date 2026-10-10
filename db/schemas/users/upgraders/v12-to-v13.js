/** Add per-account, privacy-aware, actionable new-mail notification policy. */
export default function upgrade(doc) {
  const domains = Array.isArray(doc.domains) ? [...new Set(doc.domains)] : []
  return {
    ...doc,
    newMailNotificationDomains: domains,
    newMailNotificationPrivacy: 'full',
    newMailNotificationActions: true,
  }
}
