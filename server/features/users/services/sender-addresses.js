import { Security } from '../../../platform/security/Security.js'

/**
 * Resolve the exact addresses the authenticated mailbox may use as an RFC
 * From identity. Domain authority is deliberately insufficient: another user
 * on the same hosted domain must never be able to impersonate this mailbox.
 *
 * @param {import('../repositories/UsersRepository.js').UsersRepository} users
 * @param {string} ownerEmail
 * @returns {Promise<string[]>}
 */
export async function ownedSenderAddresses(users, ownerEmail) {
  const owner = Security.normalizeEmailAddress(ownerEmail)
  if (!owner) return []
  const [, user] = await users.findByEmail(owner)
  if (!user) return [owner]
  const addresses = []
  for (const address of [user.email, ...(user.aliases ?? [])]) {
    const normalized = Security.normalizeEmailAddress(address)
    if (normalized && !addresses.includes(normalized)) addresses.push(normalized)
  }
  return addresses
}

/**
 * Normalize and authorize one requested From address against an exact mailbox
 * allowlist. Omitting the field uses the authenticated primary address.
 *
 * @param {unknown} requested
 * @param {string[]} allowed
 * @param {string} fallback
 * @returns {string | null}
 */
export function resolveOwnedSender(requested, allowed, fallback) {
  const value = typeof requested === 'string' ? requested : fallback
  const normalized = Security.normalizeEmailAddress(value)
  return normalized && allowed.includes(normalized) ? normalized : null
}
