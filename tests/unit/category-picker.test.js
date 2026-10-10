import { describe, expect, it } from 'bun:test'
import { CategoryPickerUtils } from '../../client/shared/utils/CategoryPickerUtils.js'

const buildCategoryPickerItems = (...args) => CategoryPickerUtils.buildCategoryPickerItems(...args)
const categoryLabelPatch = (...args) => CategoryPickerUtils.categoryLabelPatch(...args)
const categorySelectionState = (...args) => CategoryPickerUtils.categorySelectionState(...args)
const singleSelectionDomain = (...args) => CategoryPickerUtils.singleSelectionDomain(...args)

describe('Mailspring-compatible category picker model', () => {
  const account = {
    domain: 'example.com',
    capabilities: { folders: true, labels: true, create: true },
    categories: [
      { path: 'archive', kind: 'folder', displayName: 'Archive', depth: 0, role: 'archive' },
      { path: 'projects/launch', kind: 'folder', displayName: 'Launch', depth: 1, parentPath: 'projects' },
      { path: 'customers/enterprise', kind: 'label', displayName: 'Enterprise', depth: 1, parentPath: 'customers' },
    ],
  }

  it('filters one capability-backed account and retains nested hierarchy metadata', () => {
    expect(buildCategoryPickerItems(account, 'folder', 'launch')).toEqual([{
      path: 'projects/launch', kind: 'folder', displayName: 'Launch', depth: 1,
      parentPath: 'projects', role: '', selectedState: 'none',
    }])
    expect(buildCategoryPickerItems({ ...account, capabilities: { ...account.capabilities, labels: false } }, 'label', '')).toEqual([])
  })

  it('reports none, mixed, and all label selection across messages', () => {
    const messages = [
      { domain: 'example.com', labels: ['customers/enterprise'] },
      { domain: 'example.com', labels: [] },
    ]
    expect(categorySelectionState(messages, 'customers/enterprise')).toBe('mixed')
    expect(categorySelectionState(messages, 'missing')).toBe('none')
    expect(categorySelectionState([messages[0]], 'customers/enterprise')).toBe('all')
  })

  it('adds a mixed label to all messages and removes a label held by all', () => {
    const mixed = [{ labels: ['project'] }, { labels: [] }]
    expect(categoryLabelPatch(mixed, 'project')).toEqual({ labels: { add: ['project'], remove: [] } })
    expect(categoryLabelPatch([{ labels: ['project'] }, { labels: ['project'] }], 'project'))
      .toEqual({ labels: { add: [], remove: ['project'] } })
  })

  it('allows category actions only when every selected message belongs to one account', () => {
    expect(singleSelectionDomain([{ domain: 'EXAMPLE.com' }, { domain: 'example.com' }])).toBe('example.com')
    expect(singleSelectionDomain([{ domain: 'example.com' }, { domain: 'other.test' }])).toBe('')
    expect(singleSelectionDomain([])).toBe('')
  })
})
