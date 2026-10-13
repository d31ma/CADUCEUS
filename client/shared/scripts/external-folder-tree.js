// @ts-check

const SYSTEM_ROLE_ORDER = ['inbox', 'important', 'drafts', 'sent', 'archive', 'all', 'spam', 'trash']

/** @param {unknown} value */
function cleanText(value) { return typeof value === 'string' ? value.trim() : '' }

/** @param {string} accountId @param {string} stableId */
export function externalFolderOpenKey(accountId, stableId) {
  return `${encodeURIComponent(accountId)}:${encodeURIComponent(stableId)}`
}

/**
 * Keep the bounded provider fields used by navigation and project the flat
 * IMAP category list into Mailspring-compatible parent/child rows. A parent is
 * inferred only when that exact provider folder exists; an orphan path stays
 * a root so it never disappears behind a synthetic, non-addressable folder.
 *
 * @param {unknown} value
 * @param {string} accountId
 */
export function normalizeExternalFolders(value, accountId = '') {
  if (!Array.isArray(value)) return []
  const seenFolders = new Set()
  const source = value.slice(0, 250).flatMap(candidate => {
    if (!candidate || typeof candidate !== 'object' || Array.isArray(candidate)) return []
    const record = /** @type {Record<string, unknown>} */ (candidate)
    const stableId = cleanText(record.stableId)
    const revision = cleanText(record.revision)
    const name = cleanText(record.name)
    const role = cleanText(record.role).toLowerCase()
    const delimiter = cleanText(record.delimiter)
    const attributes = Array.isArray(record.attributes)
      ? record.attributes.slice(0, 50).map(cleanText).filter(attribute => attribute && attribute.length <= 128)
      : []
    const folder = role || name.toLowerCase()
    const unsafe = /[\0\r\n]/.test(stableId + name + delimiter)
    if (!stableId || stableId.length > 512 || !name || name.length > 1024 || role.length > 60 || delimiter.length > 4 || !folder || unsafe || seenFolders.has(folder)) return []
    seenFolders.add(folder)
    return [{
      stableId, revision, name, role, delimiter, attributes, folder, accountId,
      selectable: !attributes.some(attribute => ['\\noselect', '\\nonexistent'].includes(attribute.toLowerCase())),
      displayName: name, depth: 0, parentStableId: '', hasChildren: false,
    }]
  })

  const byName = new Map(source.map(folder => [folder.name, folder]))
  /** @type {Map<string, Array<Record<string, any>>>} */
  const children = new Map(source.map(folder => [folder.stableId, []]))
  /** @type {Array<Record<string, any>>} */
  const roots = []
  for (const folder of source) {
    const parent = findExistingParent(folder, byName)
    if (!parent) {
      roots.push(folder)
      continue
    }
    folder.parentStableId = parent.stableId
    folder.displayName = folder.name.slice(parent.name.length + folder.delimiter.length) || folder.name
    children.get(parent.stableId)?.push(folder)
  }

  /** @type {Array<Record<string, any>>} */
  const result = []
  /** @param {Record<string, any>} folder @param {number} depth */
  const visit = (folder, depth) => {
    const descendants = children.get(folder.stableId) || []
    folder.depth = depth
    folder.hasChildren = descendants.length > 0
    result.push(folder)
    descendants.sort(compareFolders).forEach(child => visit(child, depth + 1))
  }
  roots.sort(compareFolders).forEach(folder => visit(folder, 0))
  return result
}

/** @param {Record<string, any>} folder @param {Map<string, Record<string, any>>} byName */
function findExistingParent(folder, byName) {
  if (!folder.delimiter || !folder.name.includes(folder.delimiter)) return null
  let boundary = folder.name.lastIndexOf(folder.delimiter)
  while (boundary > 0) {
    const candidate = byName.get(folder.name.slice(0, boundary))
    if (candidate && candidate.stableId !== folder.stableId) return candidate
    boundary = folder.name.lastIndexOf(folder.delimiter, boundary - 1)
  }
  return null
}

/** @param {Record<string, any>} left @param {Record<string, any>} right */
function compareFolders(left, right) {
  const leftRole = SYSTEM_ROLE_ORDER.indexOf(left.role)
  const rightRole = SYSTEM_ROLE_ORDER.indexOf(right.role)
  const leftOrder = leftRole < 0 ? SYSTEM_ROLE_ORDER.length : leftRole
  const rightOrder = rightRole < 0 ? SYSTEM_ROLE_ORDER.length : rightRole
  return leftOrder - rightOrder || left.name.localeCompare(right.name, undefined, { sensitivity: 'base', numeric: true })
}

/**
 * Return depth-first rows whose complete ancestor chain is expanded.
 * Unknown keys are harmless and never cause a folder outside this account's
 * normalized catalog to render.
 *
 * @param {Array<Record<string, any>>} folders
 * @param {Record<string, boolean>} openFolders
 */
export function visibleExternalFolderRows(folders, openFolders) {
  const visible = new Set()
  return folders.filter(folder => {
    if (!folder.parentStableId) {
      visible.add(folder.stableId)
      return true
    }
    if (!visible.has(folder.parentStableId)) return false
    const parentIsOpen = openFolders?.[externalFolderOpenKey(folder.accountId, folder.parentStableId)] === true
    if (parentIsOpen) visible.add(folder.stableId)
    return parentIsOpen
  })
}

/** Derive folder actions from provider capabilities and immutable LIST data. */
export function externalFolderActions(folder, account) {
  const writable = account?.capabilities?.mailWrite === true && !['paused', 'auth_required', 'deleting'].includes(String(account?.state || ''))
  const attributes = new Set((folder?.attributes || []).map(attribute => String(attribute).toLowerCase()))
  const protectedFolder = Boolean(folder?.role) || String(folder?.name || '').toLowerCase() === 'inbox'
  return {
    createChild: writable && Boolean(folder?.delimiter) && !attributes.has('\\noinferiors'),
    rename: writable && !protectedFolder,
    delete: writable && !protectedFolder,
    export: folder?.selectable === true && String(folder?.role || '').toLowerCase() !== 'drafts',
  }
}
