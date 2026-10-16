// @ts-check

const CATEGORY_PATH = /^[a-z0-9](?:[a-z0-9._/-]{0,58}[a-z0-9])?$/

/** Match the server's bounded category-path grammar for immediate UI feedback. */
export function normalizePickerPath(value) {
  const path = String(value || '').trim().toLowerCase()
  if (!CATEGORY_PATH.test(path) || path.includes('//')) return ''
  if (path.split('/').some(part => part === '.' || part === '..')) return ''
  return path
}

/** Build safe, searchable picker rows for one account capability. */
export function buildCategoryPickerItems(account, kind, search, messages = []) {
  const capability = kind === 'label' ? 'labels' : 'folders'
  if (!account?.capabilities?.[capability]) return []
  const query = String(search || '').trim().toLowerCase()
  return (Array.isArray(account.categories) ? account.categories : [])
    .filter(category => category?.kind === kind)
    .filter(category => !query || String(category.path).toLowerCase().includes(query)
      || String(category.displayName).toLowerCase().includes(query))
    .map(category => ({
      path: String(category.path),
      kind,
      displayName: String(category.displayName || category.path),
      depth: Math.max(0, Number(category.depth) || 0),
      ...(category.parentPath ? { parentPath: String(category.parentPath) } : {}),
      role: typeof category.role === 'string' ? category.role : '',
      selectedState: kind === 'label' ? categorySelectionState(messages, category.path) : 'none',
    }))
}

/** Return whether none, some, or every selected message carries a label. */
export function categorySelectionState(messages, path) {
  const selected = Array.isArray(messages) ? messages : []
  if (selected.length === 0) return 'none'
  const usage = selected.filter(message => Array.isArray(message?.labels) && message.labels.includes(path)).length
  if (usage === 0) return 'none'
  return usage === selected.length ? 'all' : 'mixed'
}

/** Match Mailspring: a mixed label click applies it to all; an all click removes it. */
export function categoryLabelPatch(messages, path) {
  const remove = categorySelectionState(messages, path) === 'all'
  return { labels: { add: remove ? [] : [path], remove: remove ? [path] : [] } }
}

/** Resolve one exact account for a category operation, or fail closed. */
export function singleSelectionDomain(messages) {
  const domains = new Set((Array.isArray(messages) ? messages : [])
    .map(message => String(message?.domain || '').trim().toLowerCase())
    .filter(Boolean))
  return domains.size === 1 ? [...domains][0] : ''
}
