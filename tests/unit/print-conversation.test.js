import { describe, expect, it, mock } from 'bun:test'
import {
  loadPrintableConversation,
  openPrintConversation,
  renderPrintDocument,
} from '../../client/shared/scripts/print-conversation.js'

const email = (overrides = {}) => ({
  id: 'message-1',
  domain: 'example.com',
  sender: 'sender@other.com',
  recipient: 'alice@example.com',
  subject: 'Quarterly review',
  body: 'First message',
  receivedAt: '2026-01-01T10:00:00.000Z',
  folder: 'inbox',
  attachments: [],
  ...overrides,
})

describe('loadPrintableConversation', () => {
  it('loads the selected cross-folder thread from the authenticated API', async () => {
    const current = email()
    const thread = {
      subject: current.subject,
      count: 2,
      latestEmail: email({ id: 'message-2' }),
      emails: [current, email({ id: 'message-2', subject: 'Re: Quarterly review' })],
    }
    const apiFetch = mock(async () => Response.json([thread]))

    expect(await loadPrintableConversation(current, apiFetch)).toEqual(thread)
    expect(apiFetch).toHaveBeenCalledWith('/threads?emailId=message-1')
  })

  it('falls back to the open message when the thread cannot be loaded', async () => {
    const current = email()
    const apiFetch = mock(async () => { throw new Error('offline') })

    expect(await loadPrintableConversation(current, apiFetch)).toEqual({
      subject: current.subject,
      count: 1,
      latestEmail: current,
      emails: [current],
    })
  })
})

describe('renderPrintDocument', () => {
  it('renders every message and attachment in chronological order', () => {
    const document = renderPrintDocument({
      subject: 'Quarterly review',
      count: 2,
      latestEmail: email({ id: 'later' }),
      emails: [
        email({
          id: 'later',
          receivedAt: '2026-01-01T11:00:00.000Z',
          body: 'Second message',
          attachments: [{ filename: 'report.pdf', size: 2048 }],
        }),
        email({ id: 'earlier', receivedAt: '2026-01-01T10:00:00.000Z' }),
      ],
    }, { account: 'alice@example.com' })

    expect(document).toContain('<title>Quarterly review — CADUCEUS</title>')
    expect(document).toContain('/shared/assets/print.css')
    expect(document).toContain('alice@example.com')
    expect(document).toContain('2 messages')
    expect(document).toContain('report.pdf')
    expect(document).toContain('2 KB')
    expect(document.indexOf('First message')).toBeLessThan(document.indexOf('Second message'))
  })

  it('escapes untrusted fields and strips active or remote HTML content', () => {
    const document = renderPrintDocument({
      subject: '<img src=x onerror=alert(1)>',
      count: 1,
      latestEmail: email(),
      emails: [email({
        sender: '<script>alert(1)</script>@example.com',
        body: '',
        htmlBody: '<script>steal()</script><p>Hello <strong>Alice</strong></p><img src="https://tracker.example/pixel">',
        trackingId: 'secret-tracking-token',
      })],
    })

    expect(document).toContain('&lt;img src=x onerror=alert(1)&gt;')
    expect(document).toContain('Hello Alice')
    expect(document).not.toContain('<script>')
    expect(document).not.toContain('<img src=x')
    expect(document).not.toContain('tracker.example')
    expect(document).not.toContain('secret-tracking-token')
  })
})

describe('openPrintConversation', () => {
  it('writes the isolated document and opens the browser print dialog', () => {
    const written = []
    const popup = {
      opener: {},
      document: {
        readyState: 'complete',
        open: mock(() => {}),
        write: mock(value => written.push(value)),
        close: mock(() => {}),
      },
      focus: mock(() => {}),
      print: mock(() => {}),
    }
    const opener = { open: mock(() => popup) }

    expect(openPrintConversation('<!doctype html><p>Print me</p>', opener)).toBe(popup)
    expect(opener.open).toHaveBeenCalledWith('', '_blank', 'popup')
    expect(written.join('')).toContain('Print me')
    expect(popup.opener).toBeNull()
    expect(popup.print).toHaveBeenCalledTimes(1)
  })

  it('reports a blocked print window', () => {
    expect(() => openPrintConversation('<!doctype html>', { open: () => null }))
      .toThrow('The print window was blocked')
  })
})
