// @ts-check
import { ServerEnvironment } from '../shared/runtime/ServerEnvironment.js'
import { SpamFilterRules } from '../shared/delivery/SpamFilterRules.js'

/**
 * Classifies inbound mail as spam.
 *
 * SPF/DKIM/DMARC verification runs in the built-in SMTP receiver. This service
 * combines any structured verdict supplied by that receiver with conventional
 * `X-Spam-*` MIME headers and conservative content heuristics.
 *
 * Stateless aside from its configured threshold; safe to share.
 */
@Service
export default class SpamFilterService {
  threshold = SpamFilterRules.toNumber(ServerEnvironment.value('SPAM_SCORE_THRESHOLD')) ?? 5

  enabled = (ServerEnvironment.value('SPAM_FILTER_DISABLED') !== 'true')

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
    if (SpamFilterRules.isPositiveVerdict(signals.verdict)) {
      score += this.threshold
      reasons.push('upstream-verdict')
    }

    // 2. Structured upstream numeric score.
    const structuredScore = SpamFilterRules.toNumber(signals.score)
    if (structuredScore != null) {
      score += structuredScore
      reasons.push(`upstream-score=${structuredScore}`)
    }

    // 3. X-Spam-* headers from the MIME (SpamAssassin convention).
    const headerScore = SpamFilterRules.scoreFromSpamHeaders(headers, reasons)
    score += headerScore

    // 4. Conservative content heuristics — only meaningful when no upstream
    //    signal classified the message. Each contributes a fraction of the
    //    threshold so a single weak match never quarantines real mail.
    score += SpamFilterRules.contentHeuristics(signals, this.threshold, reasons)

    score = SpamFilterRules.round(score)
    return { isSpam: score >= this.threshold, score, reasons }
  }
}

