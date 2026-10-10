import { describe, expect, it } from 'bun:test'
import { GitHubNotification } from '@/features/mailbox/services/GitHubNotification.js'
import { safeGitHubTarget } from '../../client/shared/scripts/github-notification.js'

describe('Mailspring-compatible GitHub notification targets', () => {
  it('extracts a canonical GitHub link from a real GitHub sender and matching anchor text', () => {
    expect(GitHubNotification.findTarget({
      sender: 'bounce@github.com',
      headerFrom: 'GitHub <notifications@github.com>',
      htmlBody: '<a class="btn" href="https://github.com/acme/widget/pull/42?notification_referrer_id=abc&amp;view=files"><span>View it on Git&#72;ub</span></a>',
    })).toBe('https://github.com/acme/widget/pull/42?notification_referrer_id=abc&view=files')
  })

  it('accepts the concise View on GitHub label and skips an unsafe earlier candidate', () => {
    expect(GitHubNotification.findTarget({
      sender: 'notifications@github.com',
      htmlBody: '<a href="https://github.com.evil.test/steal">View it on GitHub</a><a href="https://www.github.com/acme/widget/issues/7#issuecomment-1">View on GitHub</a>',
    })).toBe('https://www.github.com/acme/widget/issues/7#issuecomment-1')
  })

  it('fails closed for sender lookalikes, unrelated labels, credentials, and non-HTTPS URLs', () => {
    const anchor = url => `<a href="${url}">View it on GitHub</a>`
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com.evil.test', htmlBody: anchor('https://github.com/acme/repo') })).toBe('')
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com', htmlBody: '<a href="https://github.com/acme/repo">Open repository</a>' })).toBe('')
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com', htmlBody: anchor('https://token@github.com/acme/repo') })).toBe('')
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com', htmlBody: anchor('http://github.com/acme/repo') })).toBe('')
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com', htmlBody: anchor('javascript:alert(1)') })).toBe('')
  })

  it('bounds HTML inspection and revalidates the final browser target', () => {
    expect(GitHubNotification.findTarget({ sender: 'notify@github.com', htmlBody: 'x'.repeat(8 * 1024 * 1024 + 1) })).toBe('')
    expect(safeGitHubTarget('https://github.com/acme/repo/issues/1')).toBe('https://github.com/acme/repo/issues/1')
    expect(safeGitHubTarget('https://github.com.evil.test/acme/repo')).toBe('')
    expect(safeGitHubTarget('https://user@github.com/acme/repo')).toBe('')
  })
})
