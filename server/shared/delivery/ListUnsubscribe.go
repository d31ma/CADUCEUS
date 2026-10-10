import (
	"net/url"
	"regexp"
	"strings"
)

// ListUnsubscribe reads RFC 2369 / RFC 8058 unsubscribe metadata into fields
// FYLO can store: an https target, a mailto target, a body link when the
// headers name none, and one-click support. Unsafe or ambiguous targets are
// dropped at delivery and never reach the browser.
type ListUnsubscribe struct{}

// Parsed answers {unsubscribeUrl, unsubscribeMailto, unsubscribeBodyUrl,
// unsubscribeOneClick}.
func (unsubscribe ListUnsubscribe) Parsed(headers map[string]any, html string) map[string]any {
	raw := Security{}.Trim(RecordValues{}.Text(headers["list-unsubscribe"]))
	httpsUrl, mailto := "", ""
	for _, match := range regexp.MustCompile(`<([^<>\r\n]{1,2048})>`).FindAllStringSubmatch(raw, -1) {
		parsed, err := url.Parse(Security{}.Trim(match[1]))
		if err != nil || parsed.Scheme == "" {
			continue
		}
		if httpsUrl == "" && parsed.Scheme == "https" && parsed.User == nil && parsed.Host != "" {
			httpsUrl = TrackingTokens{}.NormalizedUrl(parsed.String())
			continue
		}
		if mailto == "" && parsed.Scheme == "mailto" {
			mailto = unsubscribe.mailto(parsed)
		}
	}
	oneClick := Security{}.Trim(RecordValues{}.Text(headers["list-unsubscribe-post"])) == "List-Unsubscribe=One-Click"
	bodyUrl := ""
	if httpsUrl == "" && mailto == "" {
		bodyUrl = unsubscribe.bodyLink(html)
	}
	return map[string]any{"unsubscribeUrl": httpsUrl, "unsubscribeMailto": mailto, "unsubscribeBodyUrl": bodyUrl, "unsubscribeOneClick": httpsUrl != "" && oneClick}
}

func (ListUnsubscribe) mailto(parsed *url.URL) string {
	if parsed.User != nil || parsed.Host != "" {
		return ""
	}
	mailbox := parsed.Opaque
	if mailbox == "" {
		mailbox = parsed.Path
	}
	decoded, err := url.PathUnescape(mailbox)
	if err != nil || strings.Contains(decoded, ",") {
		return ""
	}
	normalized := Security{}.NormalizeEmailAddress(decoded)
	if normalized == "" {
		return ""
	}
	query := url.Values{}
	for _, key := range []string{"subject", "body"} {
		if value := parsed.Query().Get(key); parsed.Query().Has(key) && len(value) <= 2000 && !strings.ContainsAny(value, "\r\n") {
			query.Set(key, value)
		}
	}
	if len(query) == 0 {
		return "mailto:" + normalized
	}
	return "mailto:" + normalized + "?" + query.Encode()
}

// bodyLink answers the https link of an HTML body whose URL, text or
// surrounding text reads as an unsubscribe link, outside quoted replies.
func (unsubscribe ListUnsubscribe) bodyLink(html string) string {
	if html == "" || len(html) > 8_388_608 {
		return ""
	}
	withoutQuotes := regexp.MustCompile(`(?is)<blockquote\b[^>]*>.*?</blockquote\s*>`).ReplaceAllString(html, "")
	candidates := []string{}
	anchors := regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a\s*>`)
	href := regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	for _, match := range anchors.FindAllStringSubmatchIndex(withoutQuotes, -1) {
		attributes, text := withoutQuotes[match[2]:match[3]], withoutQuotes[match[4]:match[5]]
		found := href.FindStringSubmatch(attributes)
		if found == nil {
			continue
		}
		target := unsubscribe.secureHttps(unsubscribe.decodedAttribute(found[1] + found[2] + found[3]))
		if target == "" {
			continue
		}
		start, end := match[0]-256, match[1]+256
		if start < 0 {
			start = 0
		}
		if end > len(withoutQuotes) {
			end = len(withoutQuotes)
		}
		if unsubscribe.keyword(target) {
			return target
		}
		if unsubscribe.keyword(unsubscribe.stripped(text)) || unsubscribe.keyword(unsubscribe.stripped(withoutQuotes[start:end])) {
			candidates = append(candidates, target)
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func (ListUnsubscribe) secureHttps(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Host == "" {
		return ""
	}
	return TrackingTokens{}.NormalizedUrl(value)
}

func (ListUnsubscribe) keyword(value string) bool {
	pattern := `(?i)unsubscribe|unfollow|opt[ -]{0,2}out|email preferences|subscription|notification settings|remove yourself from this mailing|` +
		`afmeld|afmelden|af te melden voor|darse de baja|désabonnement|désinscrire|désinscription|désabonner|` +
		`préférences d['’]email|préférences d['’]abonnement|отказаться от подписки|одјавити|afskrá|לבטל את המנוי|` +
		`koupe abònman|退订|退訂|إلغاء الاشتراك|պետք է նախ միանալ|abbestellen|abbestellung|abmelden|` +
		`abmeldung|ausschreiben|austragen|avprenumerera|avregistrera|prenumeration|notisinställningar`
	return regexp.MustCompile(pattern).MatchString(value)
}

func (ListUnsubscribe) stripped(value string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(value, " "), " "))
}

func (ListUnsubscribe) decodedAttribute(value string) string {
	value = regexp.MustCompile(`(?i)&amp;`).ReplaceAllString(value, "&")
	value = regexp.MustCompile(`(?i)&quot;`).ReplaceAllString(value, `"`)
	return regexp.MustCompile(`(?i)&#39;|&apos;`).ReplaceAllString(value, "'")
}
