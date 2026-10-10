import { describe, expect, it } from 'bun:test'
import {
  CATEGORY_ROLES,
  defaultRoleMapping,
  normalizeCategoryMappings,
  presentEmailWithMappings,
  validateCategoryMappings,
} from '@/features/mailbox/services/CategoryMappingService.js'
import { CategoryPath } from '@/shared/mailbox/CategoryPath.js'

describe('mailbox category-role mappings', () => {
  it('normalizes only assigned domains and repairs ambiguous persisted paths', () => {
    const mappings = normalizeCategoryMappings({
      'example.com': {
        inbox: 'sent',
        sent: 'sent',
        drafts: 'system/sent',
        spam: '../unsafe',
      },
      'foreign.example': { ...defaultRoleMapping(), inbox: 'foreign-secret' },
    }, ['EXAMPLE.COM'])

    expect(Object.keys(mappings)).toEqual(['example.com'])
    expect(mappings['example.com']).toEqual({
      inbox: 'sent',
      sent: 'system/sent',
      drafts: 'drafts',
      spam: 'spam',
      archive: 'archive',
      trash: 'trash',
    })
    expect(new Set(Object.values(mappings['example.com'])).size).toBe(CATEGORY_ROLES.length)
  })

  it('accepts bounded nested provider paths and rejects traversal or malformed paths', () => {
    expect(CategoryPath.normalizeCategoryPath('  Team/Support.Inbox  ')).toBe('team/support.inbox')
    expect(CategoryPath.normalizeCategoryPath('../inbox')).toBe('')
    expect(CategoryPath.normalizeCategoryPath('team//inbox')).toBe('')
    expect(CategoryPath.normalizeCategoryPath('team/')).toBe('')
    expect(CategoryPath.normalizeCategoryPath('x'.repeat(61))).toBe('')

    const mappings = { 'example.com': { ...defaultRoleMapping(), inbox: 'team/support.inbox' } }
    expect(validateCategoryMappings(mappings, ['example.com'])).toEqual({ ok: true, mappings })
    expect(validateCategoryMappings({
      'example.com': { ...mappings['example.com'], archive: 'team/support.inbox' },
    }, ['example.com'])).toMatchObject({ ok: false })
  })

  it('presents a physical provider path with its stable logical system role', () => {
    const email = { id: 'message-1', domain: 'example.com', folder: 'Priority' }
    expect(presentEmailWithMappings(email, {
      'example.com': { ...defaultRoleMapping(), inbox: 'priority' },
    })).toEqual({ ...email, folder: 'priority', systemFolder: 'inbox' })
    expect(presentEmailWithMappings(email, {})).toEqual({
      ...email, folder: 'priority', systemFolder: null,
    })
  })
})
