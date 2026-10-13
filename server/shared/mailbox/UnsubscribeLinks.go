import (
	"net/url"
	"regexp"
	"strings"
)

// UnsubscribeLinks reads RFC 2369 and RFC 8058 unsubscribe metadata, for
// delivered and imported mail alike: the
// first safe https and mailto targets of List-Unsubscribe, whether one-click
// unsubscribe applies, and otherwise a body link that reads as an
// unsubscribe link. Unsafe targets are dropped.
type UnsubscribeLinks struct{}

func (UnsubscribeLinks) largestHtml() int { return 8_388_608 }

func (UnsubscribeLinks) keywords() *regexp.Regexp {
	return regexp.MustCompile(`(?i)unsubscribe|unfollow|opt[ -]{0,2}out|email preferences|subscription|notification settings|remove yourself from this mailing|afmeld|afmelden|af te melden voor|darse de baja|désabonnement|désinscrire|désinscription|désabonner|préférences d['’]email|préférences d['’]abonnement|отказаться от подписки|одјавити|afskrá|לבטל את המנוי|koupe abònman|退订|退訂|إلغاء الاشتراك|պետք է նախ միանալ|abbestellen|abbestellung|abmelden|abmeldung|ausschreiben|austragen|avprenumerera|avregistrera|prenumeration|notisinställningar`)
}

// Parsed answers the unsubscribe fields of a message's headers and HTML.
func (links UnsubscribeLinks) Parsed(headers map[string]any, html string) map[string]any {
	raw := TextValue{}.Trim(TextValue{}.Of(headers["list-unsubscribe"]))
	unsubscribeUrl, unsubscribeMailto := "", ""
	for _, match := range regexp.MustCompile(`<([^<>\r\n]+)>`).FindAllStringSubmatch(raw, -1) {
		if len(match[1]) > 2048 {
			continue
		}
		parsed, err := url.Parse(TextValue{}.Trim(match[1]))
		if err != nil || parsed.Scheme == "" {
			continue
		}
		scheme := strings.ToLower(parsed.Scheme)
		if unsubscribeUrl == "" && scheme == "https" && parsed.User == nil {
			_, unsubscribeUrl, _ = WebAddress{}.Parsed(match[1])
			continue
		}
		if unsubscribeMailto == "" && scheme == "mailto" {
			unsubscribeMailto = links.mailto(parsed)
		}
	}
	oneClick := TextValue{}.Trim(TextValue{}.Of(headers["list-unsubscribe-post"])) == "List-Unsubscribe=One-Click"
	body := ""
	if unsubscribeUrl == "" && unsubscribeMailto == "" {
		body = links.bodyLink(html)
	}
	return map[string]any{
		"unsubscribeUrl": unsubscribeUrl, "unsubscribeMailto": unsubscribeMailto,
		"unsubscribeBodyUrl": body, "unsubscribeOneClick": unsubscribeUrl != "" && oneClick,
	}
}

func (UnsubscribeLinks) mailto(parsed *url.URL) string {
	if parsed.User != nil || parsed.Host != "" {
		return ""
	}
	mailbox, err := url.PathUnescape(parsed.Opaque + parsed.Path)
	if err != nil || strings.Contains(mailbox, ",") {
		return ""
	}
	normalized := Security{}.NormalizeEmailAddress(mailbox)
	if normalized == "" {
		return ""
	}
	query := []string{}
	values := parsed.Query()
	for _, key := range []string{"subject", "body"} {
		if value, given := values[key]; given && (TextValue{}).Length(value[0]) <= 2_000 && !strings.ContainsAny(value[0], "\r\n") {
			query = append(query, key+"="+url.QueryEscape(value[0]))
		}
	}
	if len(query) == 0 {
		return "mailto:" + normalized
	}
	return "mailto:" + normalized + "?" + strings.Join(query, "&")
}

func (links UnsubscribeLinks) bodyLink(html string) string {
	if html == "" || (TextValue{}).Length(html) > links.largestHtml() {
		return ""
	}
	withoutQuotes := regexp.MustCompile(`(?is)<blockquote\b[^>]*>.*?</blockquote\s*>`).ReplaceAllString(html, "")
	href := regexp.MustCompile(`(?i)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	candidates := []string{}
	for _, match := range regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a\s*>`).FindAllStringSubmatchIndex(withoutQuotes, -1) {
		attributes := withoutQuotes[match[2]:match[3]]
		link := ""
		if found := href.FindStringSubmatch(attributes); found != nil {
			link = found[1] + found[2] + found[3]
		}
		target := links.secureHttps(links.decodedAttribute(link))
		if target == "" {
			continue
		}
		context := withoutQuotes[max(0, match[0]-256):min(len(withoutQuotes), match[1]+256)]
		if links.keywords().MatchString(target) {
			return target
		}
		if links.keywords().MatchString(links.stripped(withoutQuotes[match[4]:match[5]])) || links.keywords().MatchString(links.stripped(context)) {
			candidates = append(candidates, target)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func (UnsubscribeLinks) secureHttps(value string) string {
	parsed, href, valid := WebAddress{}.Parsed(value)
	if !valid || parsed.Scheme != "https" || parsed.User != nil {
		return ""
	}
	return href
}

func (UnsubscribeLinks) stripped(value string) string {
	return TextValue{}.Trim(TextValue{}.Collapsed(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(value, " ")))
}

// decodedAttribute decodes the entities an href may carry, in any letter
// case, as the former ListUnsubscribe did.
func (UnsubscribeLinks) decodedAttribute(value string) string {
	value = regexp.MustCompile(`(?i)&amp;`).ReplaceAllString(value, "&")
	value = regexp.MustCompile(`(?i)&quot;`).ReplaceAllString(value, `"`)
	return regexp.MustCompile(`(?i)&#39;|&apos;`).ReplaceAllString(value, "'")
}
