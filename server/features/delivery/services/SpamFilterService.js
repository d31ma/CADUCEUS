// @ts-check
import { ServerEnvironment } from '../../../shared/runtime/ServerEnvironment.js'

/**
 * Classifies inbound mail as spam.
 *
 * SPF/DKIM/DMARC verification runs in the built-in SMTP receiver. This service
 * combines any structured verdict supplied by that receiver with conventional
 * `X-Spam-*` MIME headers and conservative content heuristics.
 *
 * Stateless aside from its configured threshold; safe to share.
 */
export class SpamFilterService {
  /**
   * @param {{ threshold?: number, enabled?: boolean }} [options]
   */
  constructor(options = {}) {
    /** Score at/above which a message is treated as spam (SpamAssassin default 5). */
    this.threshold = options.threshold ?? toNumber(ServerEnvironment.value('SPAM_SCORE_THRESHOLD')) ?? 5
    /** Master switch — set SPAM_FILTER_DISABLED=true to pass everything through. */
    this.enabled = options.enabled ?? (ServerEnvironment.value('SPAM_FILTER_DISABLED') !== 'true')
  }

  /**
   * @param {SpamSignals} signals
   * @returns {SpamVerdict}
   */
  classify(signals = {}) {
    if (!this.enabled) return { isSpam: false, score: 0, reasons: [] }

    const headers = signals.headers ?? {}
    /** @type {string[]} */
    const reasons = []
    let score = 0

    // 1. Structured receiver verdict. A hard FAIL alone
    //    is enough to quarantine; we add the full threshold so it always trips.
    if (isPositiveVerdict(signals.verdict)) {
      score += this.threshold
      reasons.push('upstream-verdict')
    }

    // 2. Structured upstream numeric score.
    const structuredScore = toNumber(signals.score)
    if (structuredScore != null) {
      score += structuredScore
      reasons.push(`upstream-score=${structuredScore}`)
    }

    // 3. X-Spam-* headers from the MIME (SpamAssassin convention).
    const headerScore = scoreFromSpamHeaders(headers, reasons)
    score += headerScore

    // 4. Conservative content heuristics — only meaningful when no upstream
    //    signal classified the message. Each contributes a fraction of the
    //    threshold so a single weak match never quarantines real mail.
    score += contentHeuristics(signals, this.threshold, reasons)

    score = round(score)
    return { isSpam: score >= this.threshold, score, reasons }
  }
}

/** @param {string|undefined} verdict */
function isPositiveVerdict(verdict) {
  if (typeof verdict !== 'string') return false
  const v = verdict.trim().toLowerCase()
  return v === 'fail' || v === 'spam' || v === 'yes' || v === 'true'
}

/**
 * Parse the conventional SpamAssassin headers:
 *   X-Spam-Flag: YES
 *   X-Spam-Status: Yes, score=7.2 required=5.0 …
 *   X-Spam-Score: 7.2
 *
 * @param {Record<string, string>} headers
 * @param {string[]} reasons
 * @returns {number}
 */
function scoreFromSpamHeaders(headers, reasons) {
  let added = 0
  const flag = headers['x-spam-flag']
  if (typeof flag === 'string' && flag.trim().toLowerCase() === 'yes') {
    added += 5
    reasons.push('x-spam-flag')
  }
  const status = headers['x-spam-status']
  if (typeof status === 'string') {
    const m = status.match(/score=(-?\d+(?:\.\d+)?)/i)
    if (m) { added += Number(m[1]); reasons.push(`x-spam-status=${m[1]}`) }
    else if (/^yes\b/i.test(status.trim())) { added += 5; reasons.push('x-spam-status=yes') }
  }
  const xScore = toNumber(headers['x-spam-score'])
  if (xScore != null) { added += xScore; reasons.push(`x-spam-score=${xScore}`) }
  return added
}

const MONEY_WORDS = /\b(viagra|cialis|lottery|winner|jackpot|crypto|bitcoin|forex|prince|inheritance|wire transfer)\b/i
const URGENCY = /\b(act now|claim now|urgent|limited time|congratulations|you('| ha)ve won|free money|risk[- ]free)\b/i
const SUSPICIOUS_TLD = /\.(?:win|top|loan|click|gq|tk|ml|cf)$/i

/**
 * @param {SpamSignals} signals
 * @param {number} threshold
 * @param {string[]} reasons
 * @returns {number}
 */
function contentHeuristics(signals, threshold, reasons) {
  const subject = String(signals.subject ?? '')
  const text = String(signals.text ?? '')
  const sender = String(signals.sender ?? '')
  const unit = threshold * 0.4 // a single heuristic is never decisive on its own
  let added = 0

  if (URGENCY.test(subject) || URGENCY.test(text)) { added += unit; reasons.push('urgency-language') }
  if (MONEY_WORDS.test(subject) || MONEY_WORDS.test(text)) { added += unit; reasons.push('spammy-keywords') }
  if (isShouting(subject)) { added += unit * 0.75; reasons.push('shouting-subject') }
  if ((subject.match(/!/g)?.length ?? 0) >= 3) { added += unit * 0.5; reasons.push('excessive-exclamation') }
  const senderDomain = sender.split('@')[1] ?? ''
  if (SUSPICIOUS_TLD.test(senderDomain)) { added += unit; reasons.push('suspicious-tld') }

  return added
}

/** @param {string} value — true when ≥6 chars and mostly upper-case letters. */
function isShouting(value) {
  const letters = value.replace(/[^a-z]/gi, '')
  if (letters.length < 6) return false
  const upper = value.replace(/[^A-Z]/g, '').length
  return upper / letters.length >= 0.8
}

/** @param {unknown} value @returns {number | null} */
function toNumber(value) {
  if (value == null || value === '') return null
  const n = Number(value)
  return Number.isFinite(n) ? n : null
}

/** @param {number} n */
function round(n) {
  return Math.round(n * 100) / 100
}
