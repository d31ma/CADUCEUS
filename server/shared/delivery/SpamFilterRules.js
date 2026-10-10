// @ts-check
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class SpamFilterRules {
  /** @param {string|undefined} verdict */
  static isPositiveVerdict(verdict) {
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
  static scoreFromSpamHeaders(headers, reasons) {
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
    const xScore = SpamFilterRules.toNumber(headers['x-spam-score'])
    if (xScore != null) { added += xScore; reasons.push(`x-spam-score=${xScore}`) }
    return added
  }

  static MONEY_WORDS = /\b(viagra|cialis|lottery|winner|jackpot|crypto|bitcoin|forex|prince|inheritance|wire transfer)\b/i

  static URGENCY = /\b(act now|claim now|urgent|limited time|congratulations|you('| ha)ve won|free money|risk[- ]free)\b/i

  static SUSPICIOUS_TLD = /\.(?:win|top|loan|click|gq|tk|ml|cf)$/i

  /**
   * @param {SpamSignals} signals
   * @param {number} threshold
   * @param {string[]} reasons
   * @returns {number}
   */
  static contentHeuristics(signals, threshold, reasons) {
    const subject = String(signals.subject ?? '')
    const text = String(signals.text ?? '')
    const sender = String(signals.sender ?? '')
    const unit = threshold * 0.4 // a single heuristic is never decisive on its own
    let added = 0

    if (SpamFilterRules.URGENCY.test(subject) || SpamFilterRules.URGENCY.test(text)) { added += unit; reasons.push('urgency-language') }
    if (SpamFilterRules.MONEY_WORDS.test(subject) || SpamFilterRules.MONEY_WORDS.test(text)) { added += unit; reasons.push('spammy-keywords') }
    if (SpamFilterRules.isShouting(subject)) { added += unit * 0.75; reasons.push('shouting-subject') }
    if ((subject.match(/!/g)?.length ?? 0) >= 3) { added += unit * 0.5; reasons.push('excessive-exclamation') }
    const senderDomain = sender.split('@')[1] ?? ''
    if (SpamFilterRules.SUSPICIOUS_TLD.test(senderDomain)) { added += unit; reasons.push('suspicious-tld') }

    return added
  }

  /** @param {string} value — true when ≥6 chars and mostly upper-case letters. */
  static isShouting(value) {
    const letters = value.replace(/[^a-z]/gi, '')
    if (letters.length < 6) return false
    const upper = value.replace(/[^A-Z]/g, '').length
    return upper / letters.length >= 0.8
  }

  /** @param {unknown} value @returns {number | null} */
  static toNumber(value) {
    if (value == null || value === '') return null
    const n = Number(value)
    return Number.isFinite(n) ? n : null
  }

  /** @param {number} n */
  static round(n) {
    return Math.round(n * 100) / 100
  }
}
