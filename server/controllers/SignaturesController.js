import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'
import { Actions } from '../../platform/auth/bridge.js'
import { ownedSenderAddresses } from '../../features/users/services/sender-addresses.js'
import { normalizeSignatureInput, resolveUniqueSignatureName } from '../../features/mailbox/services/signature-content.js'

/** @param {import('../../features/mailbox/repositories/signatures.js').SignatureEntry} entry */
function present(entry) {
  return {
    id: entry.id,
    domain: entry.domain,
    name: entry.name,
    text: entry.text,
    defaultFor: entry.defaultFor ?? [],
    createdAt: new Date(entry.createdAt).toISOString(),
    updatedAt: new Date(entry.updatedAt || entry.createdAt).toISOString(),
  }
}

@RequestRoute("/signatures")
@Public
export class SignaturesController {
  @Guards.auth({ action: Actions.MAILBOX_WRITE })
  @PostRoute
  async post(/** @type {any} */ { body, auth }) {
    const app = await getApp()
    const addresses = await ownedSenderAddresses(app.users, auth.email)
    const input = normalizeSignatureInput(body, { allowedAddresses: addresses })
    if (input instanceof Error) return Respond.r400(input.message)
    const existing = Object.values(await app.signatures.listForOwner(auth.email))
    const now = Date.now()
    const entry = {
      id: `sig-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      owner: auth.email,
      domain: auth.email.split('@')[1] || auth.domain,
      name: resolveUniqueSignatureName(input.name ?? '', existing),
      text: input.text ?? '',
      defaultFor: input.defaultFor ?? [],
      createdAt: now,
      updatedAt: now,
    }
    await app.signatures.releaseDefaults(auth.email, entry.defaultFor, entry.id)
    await app.signatures.put(entry)
    return present(entry)
  }

  @Guards.auth({ action: Actions.MAILBOX_READ })
  @GetRoute
  async get(/** @type {any} */ { auth }) {
    const app = await getApp()
    const records = await app.signatures.listForOwner(auth.email)
    return Object.values(records).sort((left, right) => left.name.localeCompare(right.name)).map(present)
  }
}
