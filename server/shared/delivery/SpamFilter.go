import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// SpamFilter classifies inbound mail as spam by combining the structured
// verdict and score the built-in SMTP receiver supplies (SPF/DKIM/DMARC run
// there), conventional X-Spam-* MIME headers, and conservative content
// heuristics. The threshold is SPAM_SCORE_THRESHOLD (5 by default);
// SPAM_FILTER_DISABLED=true turns it off.
type SpamFilter struct{}

// Classified answers {isSpam, score, reasons} for a message's sender,
// subject, text, headers and the receiver's verdict and score.
func (filter SpamFilter) Classified(signals map[string]any) map[string]any {
	settings := Settings{}
	if settings.Value("SPAM_FILTER_DISABLED") == "true" {
		return map[string]any{"isSpam": false, "score": 0, "reasons": []string{}}
	}
	threshold, configured := filter.number(settings.Value("SPAM_SCORE_THRESHOLD"))
	if !configured {
		threshold = 5
	}
	headers, _ := RecordValues{}.Object(signals["headers"])
	reasons := []string{}
	score := 0.0
	if verdict, isText := signals["verdict"].(string); isText {
		switch strings.ToLower(strings.TrimSpace(verdict)) {
		case "fail", "spam", "yes", "true":
			score += threshold
			reasons = append(reasons, "upstream-verdict")
		}
	}
	if structured, has := filter.number(signals["score"]); has {
		score += structured
		reasons = append(reasons, "upstream-score="+filter.text(structured))
	}
	score += filter.headerScore(headers, &reasons)
	score += filter.heuristics(signals, threshold, &reasons)
	score = math.Round(score*100) / 100
	return map[string]any{"isSpam": score >= threshold, "score": score, "reasons": reasons}
}

// headerScore reads the SpamAssassin headers X-Spam-Flag, X-Spam-Status and
// X-Spam-Score.
func (filter SpamFilter) headerScore(headers map[string]any, reasons *[]string) float64 {
	added := 0.0
	if flag, isText := headers["x-spam-flag"].(string); isText && strings.ToLower(strings.TrimSpace(flag)) == "yes" {
		added += 5
		*reasons = append(*reasons, "x-spam-flag")
	}
	if status, isText := headers["x-spam-status"].(string); isText {
		if match := regexp.MustCompile(`(?i)score=(-?\d+(?:\.\d+)?)`).FindStringSubmatch(status); match != nil {
			value, _ := strconv.ParseFloat(match[1], 64)
			added += value
			*reasons = append(*reasons, "x-spam-status="+match[1])
		} else if regexp.MustCompile(`(?i)^yes\b`).MatchString(strings.TrimSpace(status)) {
			added += 5
			*reasons = append(*reasons, "x-spam-status=yes")
		}
	}
	if score, has := filter.number(headers["x-spam-score"]); has {
		added += score
		*reasons = append(*reasons, "x-spam-score="+filter.text(score))
	}
	return added
}

// heuristics adds a fraction of the threshold per weak signal, so a single
// match never quarantines real mail on its own.
func (filter SpamFilter) heuristics(signals map[string]any, threshold float64, reasons *[]string) float64 {
	values := RecordValues{}
	subject, text, sender := values.Text(signals["subject"]), values.Text(signals["text"]), values.Text(signals["sender"])
	unit := threshold * 0.4
	added := 0.0
	urgency := regexp.MustCompile(`(?i)\b(act now|claim now|urgent|limited time|congratulations|you('| ha)ve won|free money|risk[- ]free)\b`)
	money := regexp.MustCompile(`(?i)\b(viagra|cialis|lottery|winner|jackpot|crypto|bitcoin|forex|prince|inheritance|wire transfer)\b`)
	if urgency.MatchString(subject) || urgency.MatchString(text) {
		added += unit
		*reasons = append(*reasons, "urgency-language")
	}
	if money.MatchString(subject) || money.MatchString(text) {
		added += unit
		*reasons = append(*reasons, "spammy-keywords")
	}
	if filter.shouting(subject) {
		added += unit * 0.75
		*reasons = append(*reasons, "shouting-subject")
	}
	if strings.Count(subject, "!") >= 3 {
		added += unit * 0.5
		*reasons = append(*reasons, "excessive-exclamation")
	}
	if regexp.MustCompile(`(?i)\.(?:win|top|loan|click|gq|tk|ml|cf)$`).MatchString(DeliveryFields{}.Domain(sender)) {
		added += unit
		*reasons = append(*reasons, "suspicious-tld")
	}
	return added
}

// shouting reports whether a text of at least six letters is mostly upper
// case.
func (SpamFilter) shouting(value string) bool {
	letters := regexp.MustCompile(`(?i)[^a-z]`).ReplaceAllString(value, "")
	if len(letters) < 6 {
		return false
	}
	upper := regexp.MustCompile(`[^A-Z]`).ReplaceAllString(value, "")
	return float64(len(upper))/float64(len(letters)) >= 0.8
}

// number answers Number(value) for a set, finite value.
func (SpamFilter) number(value any) (float64, bool) {
	if value == nil || value == "" {
		return 0, false
	}
	number := RecordValues{}.Numeric(value)
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

func (SpamFilter) text(number float64) string {
	return strconv.FormatFloat(number, 'f', -1, 64)
}
