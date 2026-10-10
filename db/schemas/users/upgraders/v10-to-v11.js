/** Add MailSpring-compatible MCP preferences, disabled until explicit opt-in. */
export default function upgrade(doc) {
  const domains = Array.isArray(doc.domains) ? [...new Set(doc.domains)] : []
  return {
    ...doc,
    mcp: {
      enabled: false,
      accessLevel: 'read-only',
      allowedDomains: domains,
      excludedFolders: {},
    },
  }
}
