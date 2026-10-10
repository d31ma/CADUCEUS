// @ts-check

const SAFE_ID = /^[a-z0-9][a-z0-9._:-]{0,79}$/

/** Normalize one in-memory notice before it reaches the application shell. */
export function normalizeNotification(input) {
  const source = input && typeof input === 'object' && !Array.isArray(input)
    ? /** @type {Record<string, any>} */ (input)
    : {}
  const kind = SAFE_ID.test(String(source.kind || '')) ? String(source.kind) : 'custom'
  const requestedId = String(source.id || '')
  const id = SAFE_ID.test(requestedId) ? requestedId : `notice-${kind}`
  const priority = Math.max(0, Math.min(9, Math.trunc(Number(source.priority) || 0)))
  const title = String(source.title || '').slice(0, 240)
  const subtitle = String(source.subtitle || '').slice(0, 500)
  const actions = (Array.isArray(source.actions) ? source.actions : []).slice(0, 3)
    .filter(action => SAFE_ID.test(String(action?.id || '')) && String(action?.label || '').trim())
    .map(action => ({ id: String(action.id), label: String(action.label).trim().slice(0, 60) }))
  return { id, kind, priority, title, subtitle, dismissable: source.dismissable === true, actions }
}

/** Build CADUCEUS' self-hosted equivalents of Mailspring's derived notices. */
export function buildSystemNotifications(state = {}) {
  const source = /** @type {Record<string, any>} */ (state)
  const notices = []
  if (source.online === false) notices.push({
    id: 'offline', kind: 'offline', priority: 5,
    title: 'CADUCEUS is offline.', subtitle: 'Mailbox actions will synchronize after the connection returns.',
    actions: [{ id: 'try-now', label: 'Try now' }],
  })
  if (source.updateVersion) notices.push({
    id: `update-${String(source.updateVersion).replace(/[^a-z0-9._:-]/gi, '-').slice(0, 64)}`,
    kind: 'update', priority: 4, title: `A CADUCEUS update is ready (${String(source.updateVersion).slice(0, 80)}).`,
    subtitle: 'Reload to use the latest locally installed web application.', dismissable: true,
    actions: [{ id: 'reload-update', label: 'Reload' }],
  })
  const accountErrors = Array.isArray(source.accountErrors)
    ? [...new Set(source.accountErrors.map(value => String(value).trim()).filter(Boolean))].slice(0, 20)
    : []
  if (accountErrors.length) notices.push({
    id: 'account-error', kind: 'account-error', priority: 3,
    title: accountErrors.length === 1
      ? `Mail updates for ${accountErrors[0]} are reconnecting.`
      : `${accountErrors.length} mailboxes are reconnecting.`,
    subtitle: 'Stored mail remains available while CADUCEUS retries the live connection.',
    actions: [{ id: 'retry-accounts', label: 'Try again' }, { id: 'manage-accounts', label: 'Manage' }],
  })
  const disabledRuleCount = Math.max(0, Math.trunc(Number(source.disabledRuleCount) || 0))
  if (disabledRuleCount) notices.push({
    id: 'disabled-rules', kind: 'disabled-rules', priority: 2,
    title: disabledRuleCount === 1 ? 'One mail rule is disabled.' : `${disabledRuleCount} mail rules are disabled.`,
    actions: [{ id: 'view-rules', label: 'View mail rules' }],
  })
  if (source.pushPrompt === true) notices.push({
    id: 'push-prompt', kind: 'push-prompt', priority: 1,
    title: 'Enable new-mail notifications on this device?',
    subtitle: 'Notification privacy and account scope remain under your control.', dismissable: true,
    actions: [{ id: 'notification-settings', label: 'Review settings' }],
  })
  return notices.map(normalizeNotification).sort(compareNotifications)
}

/** Keep required health notices visible; filter only explicitly dismissable items. */
export function visibleNotifications(notifications, dismissedIds = []) {
  const dismissed = new Set(Array.isArray(dismissedIds) ? dismissedIds : [])
  return (Array.isArray(notifications) ? notifications : [])
    .map(normalizeNotification)
    .filter(item => !item.dismissable || !dismissed.has(item.id))
    .sort(compareNotifications)
}

export function highestPriorityNotification(notifications) {
  return visibleNotifications(notifications)[0] || null
}

function compareNotifications(left, right) {
  return right.priority - left.priority || left.id.localeCompare(right.id)
}
