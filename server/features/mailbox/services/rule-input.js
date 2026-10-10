import { Security } from '../../../platform/security/Security.js'

/**
 * Normalize the user-controlled parts of a mailbox rule before validation and
 * persistence. Ownership fields are supplied by the authenticated boundary,
 * never copied from the request.
 *
 * @param {Record<string, any>} input
 * @param {{ ownerEmail: string, mailbox: string, order: number, id?: string }} identity
 */
export function normalizeInboxRuleInput(input, identity) {
  const ownerEmail = Security.normalizeEmailAddress(identity.ownerEmail) ?? ''
  const mailbox = Security.normalizeEmailAddress(identity.mailbox) ?? ''
  return {
    ...(identity.id ? { id: identity.id } : {}),
    ownerEmail,
    mailbox,
    domain: mailbox.split('@')[1] ?? '',
    name: typeof input?.name === 'string' ? input.name.trim() : '',
    enabled: input?.enabled === undefined ? true : input.enabled,
    order: identity.order,
    stopProcessing: input?.stopProcessing === true,
    conditionMatch: input?.conditionMatch ?? 'all',
    conditions: Array.isArray(input?.conditions)
      ? input.conditions.map(normalizeCondition)
      : input?.conditions,
    actions: Array.isArray(input?.actions)
      ? input.actions.map(normalizeAction)
      : input?.actions,
  }
}

/** @param {Record<string, any>} condition */
function normalizeCondition(condition) {
  return {
    field: String(condition?.field ?? ''),
    op: String(condition?.op ?? ''),
    value: condition?.field === 'hasAttachment'
      ? String(condition?.value ?? '').trim().toLowerCase()
      : typeof condition?.value === 'string' ? condition.value.trim() : condition?.value,
  }
}

/** @param {Record<string, any>} action */
function normalizeAction(action) {
  if (action?.type === 'folder') return { type: 'folder', folder: String(action.folder ?? '').trim().toLowerCase() }
  if (action?.type === 'forward') return { type: 'forward', to: Security.normalizeEmailAddress(action.to) ?? String(action.to ?? '').trim().toLowerCase() }
  if (action?.type === 'markRead' || action?.type === 'star') return { type: action.type, value: action.value }
  return { type: String(action?.type ?? '') }
}

