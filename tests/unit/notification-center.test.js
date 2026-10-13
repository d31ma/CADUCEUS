import { describe, expect, it } from 'bun:test'
import { NotificationCenterUtils } from '../../client/shared/utils/NotificationCenterUtils.js'

const buildSystemNotifications = (...args) => NotificationCenterUtils.buildSystemNotifications(...args)
const highestPriorityNotification = (...args) => NotificationCenterUtils.highestPriorityNotification(...args)
const normalizeNotification = (...args) => NotificationCenterUtils.normalizeNotification(...args)
const visibleNotifications = (...args) => NotificationCenterUtils.visibleNotifications(...args)

describe('Mailspring-compatible notification center model', () => {
  it('builds standalone system notices with the pinned priority order', () => {
    const notices = buildSystemNotifications({
      online: false,
      updateVersion: '26.34.0',
      accountErrors: ['mail.example.com'],
      disabledRuleCount: 2,
      pushPrompt: true,
    })
    expect(notices.map(item => [item.kind, item.priority])).toEqual([
      ['offline', 5],
      ['update', 4],
      ['account-error', 3],
      ['disabled-rules', 2],
      ['push-prompt', 1],
    ])
    expect(highestPriorityNotification(notices)?.kind).toBe('offline')
  })

  it('normalizes hostile notices and retains only allow-listed actions', () => {
    expect(normalizeNotification({
      id: '../bad', kind: 'custom', priority: 99, title: '<script>',
      actions: [{ id: 'retry', label: 'Retry' }, { id: '../bad', label: 'Bad' }],
    })).toEqual({
      id: 'notice-custom', kind: 'custom', priority: 9, title: '<script>', subtitle: '',
      dismissable: false, actions: [{ id: 'retry', label: 'Retry' }],
    })
  })

  it('filters durable dismissals without allowing required health notices to disappear', () => {
    const notices = buildSystemNotifications({ online: false, updateVersion: '26.34.0', pushPrompt: true })
    expect(visibleNotifications(notices, ['offline', 'update-26.34.0', 'push-prompt']).map(item => item.kind))
      .toEqual(['offline'])
  })
})
