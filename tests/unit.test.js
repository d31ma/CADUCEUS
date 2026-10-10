/**
 * Frontend unit tests — pure functions extracted from components.
 * Run with: bun test web/tests/unit.test.js
 */
import { describe, test, expect } from 'bun:test'
import { SpamFilterService } from '../server/features/delivery/services/SpamFilterService.js'

// ── Helpers copied verbatim from components ───────────────────────────────

function initials(address) {
  const name = address.split('@')[0] || '?'
  return name.slice(0, 2).toUpperCase()
}

function formatDate(iso, now = new Date()) {
  const d = new Date(iso)
  const isToday = d.toDateString() === now.toDateString()
  if (isToday) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

// ── initials ──────────────────────────────────────────────────────────────

describe('initials', () => {
  test('returns first two chars of local part, uppercased', () => {
    expect(initials('alice@example.com')).toBe('AL')
  })

  test('works for single-char local part', () => {
    expect(initials('a@x.com')).toBe('A')
  })

  test('handles no @ symbol (treats whole string as name)', () => {
    expect(initials('bob')).toBe('BO')
  })

  test('falls back to ? for empty local part', () => {
    expect(initials('@example.com')).toBe('?')
  })

  test('uppercases lowercase input', () => {
    expect(initials('charlie@d.com')).toBe('CH')
  })
})

// ── formatDate ────────────────────────────────────────────────────────────

describe('formatDate', () => {
  test('returns time string for today', () => {
    const now = new Date()
    const result = formatDate(now.toISOString(), now)
    // Should be a time like "2:30 PM" or "14:30"
    expect(result).toMatch(/\d/)
    expect(result).not.toMatch(/Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/)
  })

  test('returns date string for a different day', () => {
    const yesterday = new Date()
    yesterday.setDate(yesterday.getDate() - 1)
    const result = formatDate(yesterday.toISOString(), new Date())
    // Should be like "Apr 11" — contains month abbreviation
    expect(result).toMatch(/Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/)
  })

  test('handles ISO timestamp with time component', () => {
    const now = new Date()
    const iso = now.toISOString()
    const result = formatDate(iso, now)
    expect(typeof result).toBe('string')
    expect(result.length).toBeGreaterThan(0)
  })
})

describe('spam filter classify', () => {
  const filter = new SpamFilterService({ threshold: 5, enabled: true })

  test('passes ordinary mail', () => {
    const v = filter.classify({ sender: 'bob@friend.com', subject: 'Lunch tomorrow?', text: 'Want to grab lunch next week?' })
    expect(v.isSpam).toBe(false)
    expect(v.score).toBeLessThan(5)
  })

  test('honors a structured receiver FAIL verdict regardless of content', () => {
    const v = filter.classify({ sender: 'store@shop.com', subject: 'Your receipt', text: 'Thanks!', verdict: 'FAIL' })
    expect(v.isSpam).toBe(true)
    expect(v.reasons).toContain('upstream-verdict')
  })

  test('adds a structured receiver score', () => {
    const v = filter.classify({ sender: 'a@b.com', subject: 'hi', text: 'hi', score: 9 })
    expect(v.isSpam).toBe(true)
    expect(v.score).toBe(9)
  })

  test('reads X-Spam-Flag and X-Spam-Status headers', () => {
    const v = filter.classify({ sender: 'a@b.com', subject: 'x', text: 'x', headers: { 'x-spam-flag': 'YES', 'x-spam-status': 'Yes, score=7.2 required=5.0' } })
    expect(v.isSpam).toBe(true)
    expect(v.reasons).toContain('x-spam-flag')
  })

  test('catches obvious heuristic junk with no structured signal', () => {
    const v = filter.classify({
      sender: 'win@lottery-prize.win',
      subject: 'CONGRATULATIONS YOU HAVE WON!!!',
      text: 'Claim now! Wire transfer your bitcoin to collect the jackpot.',
    })
    expect(v.isSpam).toBe(true)
    expect(v.reasons.length).toBeGreaterThan(1)
  })

  test('a single weak heuristic never quarantines on its own', () => {
    const v = filter.classify({ sender: 'newsletter@store.com', subject: 'Act now on your order', text: 'Your shipment is ready.' })
    expect(v.isSpam).toBe(false)
  })

  test('disabled filter passes everything', () => {
    const off = new SpamFilterService({ enabled: false })
    expect(off.classify({ sender: 'x@x.win', subject: 'WON LOTTERY JACKPOT', text: 'bitcoin', verdict: 'FAIL' }).isSpam).toBe(false)
  })
})
