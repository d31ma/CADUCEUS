import "strings"

// QuotedMessageUtils writes the quoted original that a reply or forward
// starts with, as quotedMessageHTML in rich-compose-editor.js did.
type QuotedMessageUtils struct{}

func (QuotedMessageUtils) Html(quoted QuotedMessage) string {
	sender := QuotedMessageUtils{}.escaped(quoted.Sender)
	lineBreak := "<br>"
	body := strings.ReplaceAll(strings.ReplaceAll(QuotedMessageUtils{}.escaped(quoted.Body), "\r\n", lineBreak), "\n", lineBreak)
	heading := QuotedMessageUtils{}.heading(quoted.Mode, sender, QuotedMessageUtils{}.escaped(quoted.DateLabel), QuotedMessageUtils{}.escaped(quoted.Subject))
	return `<div><br></div><section data-caduceus-quote="` + quoted.Mode + `">` + heading + `<blockquote>` + body + `</blockquote></section>`
}

func (QuotedMessageUtils) escaped(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(value)
}

func (QuotedMessageUtils) heading(mode string, sender string, date string, subject string) string {
	if mode == "forward" {
		return `<p><strong>---------- Forwarded message ----------</strong><br>From: ` + sender + `<br>Date: ` + date + `<br>Subject: ` + subject + `</p>`
	}
	return `<p>On ` + date + `, ` + sender + ` wrote:</p>`
}
