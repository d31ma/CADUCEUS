import (
	"crypto/tls"
	"errors"
	"net"
	"net/smtp"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SmtpDirect delivers straight to each recipient domain's MX hosts on port
// 25, without a relay, as the former SmtpDirectAdapter did with nodemailer:
// STARTTLS when offered (required under SMTP_TLS_POLICY=require), DKIM when
// configured, one session per domain (or per recipient for tracked mail,
// whose body carries recipient-specific capabilities).
type SmtpDirect struct{}

// SendEmail delivers a mailbox's message; Reply-To defaults to the sender.
func (direct SmtpDirect) SendEmail(from string, message map[string]any) (map[string]any, error) {
	composed := DeliveryFields{}.Copy(message)
	if composed["replyTo"] == nil {
		composed["replyTo"] = []any{from}
	}
	return direct.Send(composed, from)
}

// Send delivers a message from fromOverride, else SMTP_FROM, else
// postmaster@MAIL_HOSTNAME.
func (direct SmtpDirect) Send(message map[string]any, fromOverride string) (map[string]any, error) {
	settings := Settings{}
	hostname := settings.Value("MAIL_HOSTNAME")
	if hostname == "" {
		hostname = "localhost"
	}
	from := fromOverride
	if from == "" {
		from = settings.Value("SMTP_FROM")
	}
	if from == "" {
		from = "postmaster@" + hostname
	}
	recipients := direct.uniqueRecipients(OutboundQueue{}.Requested(message))
	tracked := RecordValues{}.Text(message["trackingId"]) != "" && (message["openTracking"] == true || message["linkTracking"] == true)
	results := []map[string]any{}
	for _, delivery := range direct.plan(recipients, tracked) {
		results = append(results, direct.deliverToDomain(delivery, from, message, hostname))
	}
	messageId := ""
	for _, result := range results {
		if len(DeliveryFields{}.Strings(result["failedRecipients"])) > 0 {
			return nil, SmtpFailure{}.FromResults(results)
		}
		if messageId == "" && len(DeliveryFields{}.Strings(result["deliveredRecipients"])) > 0 {
			messageId = RecordValues{}.Text(result["messageId"])
		}
	}
	if messageId == "" {
		messageId = "direct-" + strconv.FormatInt(EpochTime{}.Now(), 10)
	}
	return map[string]any{"messageId": messageId, "results": results}, nil
}

// plan answers one delivery per domain, or one per recipient when tracked.
func (direct SmtpDirect) plan(recipients []string, tracked bool) []map[string]any {
	deliveries := []map[string]any{}
	if tracked {
		for _, recipient := range recipients {
			deliveries = append(deliveries, map[string]any{"domain": strings.ToLower(DeliveryFields{}.Domain(recipient)), "recipients": []string{recipient}, "trackedRecipient": recipient})
		}
		return deliveries
	}
	order, grouped := []string{}, map[string][]string{}
	for _, recipient := range recipients {
		domain := strings.ToLower(DeliveryFields{}.Domain(recipient))
		if domain == "" {
			continue
		}
		if _, seen := grouped[domain]; !seen {
			order = append(order, domain)
		}
		grouped[domain] = append(grouped[domain], recipient)
	}
	for _, domain := range order {
		deliveries = append(deliveries, map[string]any{"domain": domain, "recipients": grouped[domain]})
	}
	return deliveries
}

func (direct SmtpDirect) deliverToDomain(delivery map[string]any, from string, message map[string]any, hostname string) map[string]any {
	domain := RecordValues{}.Text(delivery["domain"])
	if domain == "" {
		return direct.failed(delivery, nil)
	}
	hosts, err := direct.mailExchangers(domain)
	if err != nil {
		return direct.failed(delivery, err)
	}
	source, err := direct.source(from, message, RecordValues{}.Text(delivery["trackedRecipient"]))
	if err != nil {
		return direct.failed(delivery, err)
	}
	var last error
	for _, host := range hosts {
		accepted, rejected, err := direct.transfer(host, hostname, from, delivery["recipients"].([]string), source)
		if err == nil {
			return direct.accepted(delivery, host, accepted, rejected, RecordValues{}.Text(message["messageId"]))
		}
		last = err
	}
	return direct.failed(delivery, last)
}

// source answers the composed (and, when configured, DKIM-signed) message,
// with the tracked body of one recipient when there is one.
func (direct SmtpDirect) source(from string, message map[string]any, trackedRecipient string) ([]byte, error) {
	rendered := message
	trackingId := RecordValues{}.Text(message["trackingId"])
	if trackedRecipient != "" && trackingId != "" && (message["openTracking"] == true || message["linkTracking"] == true) {
		tracked := TrackedHtml{TrackingId: trackingId, Recipient: trackedRecipient, OpenTracking: message["openTracking"] == true, LinkTracking: message["linkTracking"] == true}
		html, err := tracked.Prepared(RecordValues{}.Text(message["html"]))
		if err != nil {
			return nil, err
		}
		rendered = DeliveryFields{}.Merged(message, map[string]any{"html": html})
	}
	source := MimeMessage{}.Source(from, rendered, RecordValues{}.Text(message["messageId"]), time.Now())
	if signer, configured := (DkimSigner{}).FromSettings(); configured {
		return signer.Signed(source)
	}
	return source, nil
}

// transfer runs one SMTP session and answers the accepted recipients and the
// rejected ones ({recipient, responseCode, error}).
func (direct SmtpDirect) transfer(host string, hostname string, from string, recipients []string, source []byte) ([]string, []map[string]any, error) {
	policy := Settings{}.Value("SMTP_TLS_POLICY")
	if policy == "" {
		policy = "opportunistic"
	}
	if policy != "opportunistic" && policy != "require" {
		return nil, nil, &SmtpFailure{Message: "SMTP_TLS_POLICY must be opportunistic or require", ResponseCode: 550}
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, "25"), 20*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Minute))
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return nil, nil, direct.coded(err)
	}
	defer client.Close()
	if err := client.Hello(hostname); err != nil {
		return nil, nil, direct.coded(err)
	}
	if offered, _ := client.Extension("STARTTLS"); offered {
		if err := client.StartTLS(&tls.Config{ServerName: host, InsecureSkipVerify: policy != "require"}); err != nil {
			return nil, nil, direct.coded(err)
		}
	} else if policy == "require" {
		return nil, nil, &SmtpFailure{Message: "Remote server does not offer STARTTLS", ResponseCode: 451}
	}
	if err := client.Mail(from); err != nil {
		return nil, nil, direct.coded(err)
	}
	accepted, rejected := []string{}, []map[string]any{}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			coded := direct.coded(err).(*SmtpFailure)
			rejected = append(rejected, map[string]any{"recipient": recipient, "responseCode": coded.ResponseCode, "error": direct.safeError(coded.Message)})
			continue
		}
		accepted = append(accepted, recipient)
	}
	if len(accepted) == 0 {
		code := 451
		if len(rejected) > 0 {
			code = rejected[len(rejected)-1]["responseCode"].(int)
		}
		return nil, nil, &SmtpFailure{Message: "Can't send mail - all recipients were rejected", ResponseCode: code}
	}
	writer, err := client.Data()
	if err != nil {
		return nil, nil, direct.coded(err)
	}
	if _, err := writer.Write(source); err != nil {
		return nil, nil, direct.coded(err)
	}
	if err := writer.Close(); err != nil {
		return nil, nil, direct.coded(err)
	}
	_ = client.Quit()
	return accepted, rejected, nil
}

// coded answers an SMTP error as a *SmtpFailure with its reply code.
func (SmtpDirect) coded(err error) error {
	var reply *textproto.Error
	if errors.As(err, &reply) {
		return &SmtpFailure{Message: strconv.Itoa(reply.Code) + " " + reply.Msg, ResponseCode: reply.Code}
	}
	var failure *SmtpFailure
	if errors.As(err, &failure) {
		return failure
	}
	return &SmtpFailure{Message: err.Error(), ResponseCode: 451}
}

// mailExchangers answers the domain's MX hosts by preference, the domain
// itself when it publishes none, a 556 failure for a null MX, or a 451 when
// the lookup fails temporarily.
func (SmtpDirect) mailExchangers(domain string) ([]string, error) {
	records, err := net.LookupMX(domain)
	if err != nil {
		var dnsError *net.DNSError
		if errors.As(err, &dnsError) && dnsError.IsNotFound {
			return []string{domain}, nil
		}
		return nil, &SmtpFailure{Message: "MX lookup temporarily failed.", ResponseCode: 451}
	}
	sort.SliceStable(records, func(left, right int) bool { return records[left].Pref < records[right].Pref })
	hosts := []string{}
	for _, record := range records {
		if record.Host == "." || record.Host == "" {
			return nil, &SmtpFailure{Message: "Domain publishes a null MX and does not accept email.", ResponseCode: 556}
		}
		hosts = append(hosts, strings.TrimSuffix(record.Host, "."))
	}
	if len(hosts) == 0 {
		return []string{domain}, nil
	}
	return hosts, nil
}

func (direct SmtpDirect) accepted(delivery map[string]any, host string, accepted []string, rejected []map[string]any, messageId string) map[string]any {
	requested := delivery["recipients"].([]string)
	acceptedSet := map[string]bool{}
	for _, address := range accepted {
		acceptedSet[strings.ToLower(address)] = true
	}
	failed, failures := []string{}, []any{}
	permanent := true
	for _, recipient := range requested {
		if acceptedSet[strings.ToLower(recipient)] {
			continue
		}
		failed = append(failed, recipient)
		failure := map[string]any{"recipient": recipient, "responseCode": 451, "error": "recipient rejected"}
		for _, rejection := range rejected {
			if strings.EqualFold(RecordValues{}.Text(rejection["recipient"]), recipient) {
				failure = rejection
			}
		}
		permanent = permanent && direct.permanent(failure["responseCode"])
		failures = append(failures, failure)
	}
	result := map[string]any{
		"domain": delivery["domain"], "recipients": requested, "deliveredRecipients": accepted, "failedRecipients": failed,
		"host": host, "messageId": messageId, "accepted": accepted, "rejected": failed,
	}
	if recipient, tracked := delivery["trackedRecipient"]; tracked {
		result["recipient"] = recipient
	}
	if len(failed) > 0 {
		code := 451
		if permanent {
			code = 550
		}
		result["error"], result["responseCode"], result["recipientFailures"] = "Remote server rejected one or more recipients.", code, failures
	}
	return result
}

func (direct SmtpDirect) failed(delivery map[string]any, err error) map[string]any {
	code, reason := 451, "delivery failed"
	if err != nil {
		reason = direct.safeError(err.Error())
		var failure *SmtpFailure
		if errors.As(err, &failure) && failure.ResponseCode != 0 {
			code = failure.ResponseCode
		}
	}
	result := map[string]any{
		"domain": delivery["domain"], "recipients": delivery["recipients"], "deliveredRecipients": []string{},
		"failedRecipients": delivery["recipients"], "error": reason, "responseCode": code,
	}
	if recipient, tracked := delivery["trackedRecipient"]; tracked {
		result["recipient"] = recipient
	}
	return result
}

// recipientFailures answers a result's per-recipient failures.
func (SmtpDirect) recipientFailures(result map[string]any) []map[string]any {
	failures := []map[string]any{}
	if listed, isList := (RecordValues{}).List(result["recipientFailures"]); isList && len(listed) > 0 {
		for _, item := range listed {
			if failure, isObject := (RecordValues{}).Object(item); isObject {
				failures = append(failures, failure)
			}
		}
		return failures
	}
	for _, recipient := range (DeliveryFields{}).Strings(result["failedRecipients"]) {
		failures = append(failures, map[string]any{"recipient": recipient, "responseCode": result["responseCode"]})
	}
	return failures
}

// permanent reports whether an SMTP reply code is a 5xx.
func (SmtpDirect) permanent(value any) bool {
	code := RecordValues{}.Numeric(value)
	return code >= 500 && code < 600
}

func (SmtpDirect) safeError(value string) string {
	if value == "" {
		value = "delivery failed"
	}
	return TextValue{}.Prefix(regexp.MustCompile(`[\r\n\x00]+`).ReplaceAllString(value, " "), 500)
}

// uniqueRecipients answers the addresses without blanks and case-insensitive
// repeats, in order.
func (SmtpDirect) uniqueRecipients(addresses []string) []string {
	seen, unique := map[string]bool{}, []string{}
	for _, address := range addresses {
		normalized := strings.ToLower(strings.TrimSpace(address))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		unique = append(unique, address)
	}
	return unique
}
