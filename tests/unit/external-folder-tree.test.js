import { describe, expect, it } from 'bun:test'
import {
  externalFolderActions,
  externalFolderOpenKey,
  normalizeExternalFolders,
  visibleExternalFolderRows,
} from '../../client/shared/scripts/external-folder-tree.js'

describe('connected-account folder hierarchy', () => {
  const accountId = 'ext_abcdefghijklmnopqrst'
  const folders = [
    { stableId: 'INBOX', revision: '4', name: 'INBOX', role: 'inbox', delimiter: '/', attributes: [] },
    { stableId: 'Projects', revision: '2', name: 'Projects', role: '', delimiter: '/', attributes: [] },
    { stableId: 'Projects/Launch', revision: '3', name: 'Projects/Launch', role: '', delimiter: '/', attributes: [] },
    { stableId: 'Projects/Launch/Design', revision: '1', name: 'Projects/Launch/Design', role: '', delimiter: '/', attributes: [] },
    { stableId: 'Projects/Support', revision: '1', name: 'Projects/Support', role: '', delimiter: '/', attributes: [] },
    { stableId: 'Orphan/Child', revision: '1', name: 'Orphan/Child', role: '', delimiter: '/', attributes: [] },
  ]

  it('normalizes bounded provider fields and orders parents before descendants', () => {
    expect(normalizeExternalFolders(folders, accountId).map(folder => ({
      stableId: folder.stableId,
      displayName: folder.displayName,
      depth: folder.depth,
      parentStableId: folder.parentStableId,
      hasChildren: folder.hasChildren,
    }))).toEqual([
      { stableId: 'INBOX', displayName: 'INBOX', depth: 0, parentStableId: '', hasChildren: false },
      { stableId: 'Orphan/Child', displayName: 'Orphan/Child', depth: 0, parentStableId: '', hasChildren: false },
      { stableId: 'Projects', displayName: 'Projects', depth: 0, parentStableId: '', hasChildren: true },
      { stableId: 'Projects/Launch', displayName: 'Launch', depth: 1, parentStableId: 'Projects', hasChildren: true },
      { stableId: 'Projects/Launch/Design', displayName: 'Design', depth: 2, parentStableId: 'Projects/Launch', hasChildren: false },
      { stableId: 'Projects/Support', displayName: 'Support', depth: 1, parentStableId: 'Projects', hasChildren: false },
    ])
  })

  it('shows descendants only while every ancestor is expanded', () => {
    const normalized = normalizeExternalFolders(folders, accountId)
    expect(visibleExternalFolderRows(normalized, {}).map(folder => folder.stableId)).toEqual([
      'INBOX', 'Orphan/Child', 'Projects',
    ])

    const projectsKey = externalFolderOpenKey(accountId, 'Projects')
    expect(visibleExternalFolderRows(normalized, { [projectsKey]: true }).map(folder => folder.stableId)).toEqual([
      'INBOX', 'Orphan/Child', 'Projects', 'Projects/Launch', 'Projects/Support',
    ])

    const launchKey = externalFolderOpenKey(accountId, 'Projects/Launch')
    expect(visibleExternalFolderRows(normalized, { [projectsKey]: true, [launchKey]: true }).map(folder => folder.stableId)).toEqual([
      'INBOX', 'Orphan/Child', 'Projects', 'Projects/Launch', 'Projects/Launch/Design', 'Projects/Support',
    ])
  })

  it('retains non-selectable hierarchy roots while rejecting malformed and duplicate rows', () => {
    expect(normalizeExternalFolders([
      { stableId: 'NoSelect', name: 'NoSelect', delimiter: '/', attributes: ['\\Noselect'] },
      { stableId: 'ok', name: 'Folder', delimiter: '.', attributes: [] },
      { stableId: 'duplicate', name: 'Folder', delimiter: '.', attributes: [] },
      { stableId: 'bad\nname', name: 'Bad', delimiter: '/', attributes: [] },
      { stableId: 'too-wide', name: 'x'.repeat(1025), delimiter: '/', attributes: [] },
    ], accountId)).toEqual([
      expect.objectContaining({ stableId: 'ok', name: 'Folder', delimiter: '.', selectable: true }),
      expect.objectContaining({ stableId: 'NoSelect', name: 'NoSelect', selectable: false }),
    ])
  })

  it('protects system folders while exposing writable custom-folder lifecycle actions', () => {
    const [inbox, , projects, launch] = normalizeExternalFolders(folders, accountId)
    const writable = { capabilities: { mailWrite: true }, state: 'idle' }
    expect(externalFolderActions(inbox, writable)).toEqual({ createChild: true, rename: false, delete: false, export: true })
    expect(externalFolderActions(projects, writable)).toEqual({ createChild: true, rename: true, delete: true, export: true })
    expect(externalFolderActions({ ...launch, attributes: ['\\Noinferiors'] }, writable).createChild).toBe(false)
    expect(externalFolderActions(projects, { ...writable, state: 'paused' })).toEqual({ createChild: false, rename: false, delete: false, export: true })
    expect(externalFolderActions({ ...projects, role: 'drafts' }, writable).export).toBe(false)
  })
})
