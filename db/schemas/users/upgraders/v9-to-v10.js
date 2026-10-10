const roles = ['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash']

/** Materialize exact-domain category-role defaults for every mailbox. */
export default function upgrade(doc) {
  const existing = doc.categoryMappings && typeof doc.categoryMappings === 'object'
    ? doc.categoryMappings
    : {}
  return {
    ...doc,
    categoryMappings: Object.fromEntries((Array.isArray(doc.domains) ? doc.domains : []).map(domain => [
      domain,
      Object.fromEntries(roles.map(role => [role, existing?.[domain]?.[role] || role])),
    ])),
  }
}
