import (
	"fmt"
	"strings"
)

// @Service
type DomainRecordService struct{}

func (records *DomainRecordService) Marks(dns DnsState) []DnsMark {
	return records.marks(records.plans(Domain{Dns: dns}))
}

func (records *DomainRecordService) MarksLabel(dns DnsState) string {
	return records.marksLabel(records.okCount(dns), records.markWords(records.Marks(dns)))
}

func (records *DomainRecordService) Records(domain Domain) []DnsRecord {
	return records.recordsOf(records.plans(domain))
}

func (records *DomainRecordService) Status(dns DnsState) string {
	return fmt.Sprintf("%d of %d records are OK.", records.okCount(dns), records.recordCount())
}

func (records *DomainRecordService) Waiting(dns DnsState) string {
	mx := fmt.Sprintf(records.look(dns.Mx).Sentence, records.mx())
	dkim := fmt.Sprintf(records.look(dns.Dkim).Sentence, records.dkim())
	return fmt.Sprintf("%s %s Create or fix the records below, then check again. People can join this domain after mail is accepted.", mx, dkim)
}

func (records *DomainRecordService) plans(domain Domain) []DnsRecordPlan {
	return []DnsRecordPlan{
		{Key: "mx", Name: records.mx(), Type: records.mx(), Need: "Required to receive mail", Host: domain.Name, State: domain.Dns.Mx},
		{Key: "spf", Name: "SPF", Type: records.textType(), Need: "Needed so other servers trust your mail", Host: domain.Name, State: domain.Dns.Spf},
		records.dkimPlan(domain),
		{Key: "dmarc", Name: "DMARC", Type: records.textType(), Need: "Recommended: tells other servers what to do with fake mail", Host: "_dmarc." + domain.Name, State: domain.Dns.Dmarc},
	}
}

func (records *DomainRecordService) dkimPlan(domain Domain) DnsRecordPlan {
	selector := domain.DkimSelector
	return DnsRecordPlan{
		Key:      "dkim",
		Name:     records.dkim(),
		Type:     records.textType(),
		Need:     "Required to receive and sign mail · selector " + selector,
		Host:     selector + "._domainkey." + domain.Name,
		Selector: selector,
		State:    domain.Dns.Dkim,
	}
}

func (records *DomainRecordService) mx() string {
	return "MX"
}

func (records *DomainRecordService) dkim() string {
	return "DKIM"
}

func (records *DomainRecordService) textType() string {
	return "TXT"
}

func (records *DomainRecordService) recordsOf(plans []DnsRecordPlan) []DnsRecord {
	list := []DnsRecord{}
	for _, plan := range plans {
		list = append(list, records.record(plan))
	}
	return list
}

func (records *DomainRecordService) record(plan DnsRecordPlan) DnsRecord {
	look := records.look(plan.State)
	return DnsRecord{
		Key:         plan.Key,
		TitleId:     "record-" + plan.Key,
		Title:       plan.Name + " record",
		Type:        plan.Type,
		Need:        plan.Need,
		Host:        plan.Host,
		Selector:    plan.Selector,
		HasSelector: plan.Selector != "",
		TagColor:    look.Color,
		TagVariant:  look.Variant,
		TagIcon:     look.Icon,
		TagText:     look.TagText,
		CardClass:   look.CardClass,
		ResultClass: look.ResultClass,
		ResultIcon:  look.ResultIcon,
		ResultText:  strings.NewReplacer("{type}", plan.Type, "{host}", plan.Host).Replace(look.ResultFormat),
		CopyLabel:   fmt.Sprintf("Copy %s host", plan.Name),
		CopiedLabel: fmt.Sprintf("Copied %s host", plan.Name),
	}
}

func (records *DomainRecordService) marks(plans []DnsRecordPlan) []DnsMark {
	list := []DnsMark{}
	for _, plan := range plans {
		look := records.look(plan.State)
		list = append(list, DnsMark{Name: plan.Name, Color: look.Color, Variant: look.Variant, Icon: look.Icon, Words: look.Words})
	}
	return list
}

func (records *DomainRecordService) marksLabel(okCount int, words string) string {
	if okCount == records.recordCount() {
		return "DNS: all 4 records OK"
	}
	return "DNS: " + words
}

func (records *DomainRecordService) markWords(marks []DnsMark) string {
	parts := []string{}
	for _, mark := range marks {
		parts = append(parts, fmt.Sprintf("%s %s", mark.Name, mark.Words))
	}
	return strings.Join(parts, ", ")
}

func (records *DomainRecordService) okCount(dns DnsState) int {
	count := 0
	for _, state := range []string{dns.Mx, dns.Spf, dns.Dkim, dns.Dmarc} {
		count += records.look(state).Ok
	}
	return count
}

func (records *DomainRecordService) recordCount() int {
	return 4
}

func (records *DomainRecordService) look(state string) DnsLook {
	return records.looks()[records.knownState(state)]
}

func (records *DomainRecordService) knownState(state string) string {
	_, known := records.looks()[state]
	if known {
		return state
	}
	return ""
}

func (records *DomainRecordService) looks() map[string]DnsLook {
	return map[string]DnsLook{
		"ok":                   records.okLook(),
		records.missingState(): records.missingLook(),
		"wrong":                records.wrongLook(),
		"":                     records.uncheckedLook(),
	}
}

func (records *DomainRecordService) missingState() string {
	return "missing"
}

func (records *DomainRecordService) okLook() DnsLook {
	return DnsLook{
		Color:        "success",
		Icon:         records.checkIcon(),
		Words:        records.okWord(),
		TagText:      records.okWord(),
		CardClass:    records.recordCard(),
		ResultClass:  "result result--ok",
		ResultIcon:   records.checkIcon(),
		ResultFormat: "Found the right value.",
		Sentence:     "%s is OK.",
		Ok:           1,
	}
}

func (records *DomainRecordService) okWord() string {
	return "OK"
}

func (records *DomainRecordService) checkIcon() string {
	return "check"
}

func (records *DomainRecordService) missingLook() DnsLook {
	return DnsLook{
		Color:        "error",
		Icon:         "close",
		Words:        records.missingState(),
		TagText:      "Missing",
		CardClass:    "record-card record-card--bad",
		ResultClass:  records.badResult(),
		ResultIcon:   records.alertIcon(),
		ResultFormat: "No {type} record found at {host}. Create it, then select Check again.",
		Sentence:     "%s is missing.",
	}
}

func (records *DomainRecordService) wrongLook() DnsLook {
	return DnsLook{
		Color:        "warning",
		Icon:         records.alertIcon(),
		Words:        "wrong value",
		TagText:      "Wrong value",
		CardClass:    "record-card record-card--wrong",
		ResultClass:  records.badResult(),
		ResultIcon:   records.alertIcon(),
		ResultFormat: "The record at {host} has a different value. Fix it at your DNS provider, then select Check again.",
		Sentence:     "%s has a wrong value.",
	}
}

func (records *DomainRecordService) alertIcon() string {
	return "alert"
}

func (records *DomainRecordService) badResult() string {
	return "result result--bad"
}

func (records *DomainRecordService) uncheckedLook() DnsLook {
	return DnsLook{
		Variant:      "outlined",
		Icon:         "minus-circle",
		Words:        "not checked",
		TagText:      "Not checked",
		CardClass:    records.recordCard(),
		ResultClass:  "result",
		ResultIcon:   "clock",
		ResultFormat: "Not checked yet. Select Check again.",
		Sentence:     "%s is not checked yet.",
	}
}

func (records *DomainRecordService) recordCard() string {
	return "record-card"
}
