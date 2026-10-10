import { normalizeCategoryPath } from './category-path.js'

const MAX_LABELS = 100

/** Validate a public add/remove label operation. */
export function parseLabelOperation(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'labels must contain add or remove arrays' }
  }
  const unknown = Object.keys(value).filter(key => key !== 'add' && key !== 'remove')
  if (unknown.length > 0) return { ok: false, error: `Unsupported labels field: ${unknown[0]}` }
  const add = normalizeRequestedList(value.add)
  const remove = normalizeRequestedList(value.remove)
  if (!add.ok) return add
  if (!remove.ok) return remove
  const addLabels = add.labels || []
  const removeLabels = remove.labels || []
  if (addLabels.length + removeLabels.length === 0) {
    return { ok: false, error: 'labels must add or remove at least one path' }
  }
  if (addLabels.length + removeLabels.length > MAX_LABELS) {
    return { ok: false, error: `labels cannot contain more than ${MAX_LABELS} paths` }
  }
  if (addLabels.some(path => removeLabels.includes(path))) {
    return { ok: false, error: 'A label cannot be added and removed together' }
  }
  return { ok: true, operation: { add: addLabels, remove: removeLabels } }
}

/** Apply one normalized operation while preserving stable label order. */
export function applyLabelOperation(current, operation) {
  const remove = new Set(operation.remove)
  const labels = normalizeStoredLabels(current).filter(label => !remove.has(label))
  for (const label of operation.add) if (!labels.includes(label)) labels.push(label)
  return labels
}

/** Defensively normalize labels read from FYLO or an action journal. */
export function normalizeStoredLabels(value) {
  if (!Array.isArray(value)) return []
  return [...new Set(value.map(normalizeCategoryPath).filter(Boolean))].slice(0, MAX_LABELS)
}

function normalizeRequestedList(value) {
  if (value === undefined) return { ok: true, labels: [] }
  if (!Array.isArray(value)) return { ok: false, error: 'labels add and remove must be arrays' }
  const labels = value.map(normalizeCategoryPath)
  if (labels.some(path => !path)) return { ok: false, error: 'labels contain an invalid category path' }
  if (new Set(labels).size !== labels.length) return { ok: false, error: 'labels cannot contain duplicates' }
  return { ok: true, labels }
}
