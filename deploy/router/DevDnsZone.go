package main

import (
	"regexp"
	"strings"
)

type DevDnsZone struct {
	Settings DevDnsSettings
}

func (zone DevDnsZone) Response(name string, kind string) DevDnsResponse {
	asked := zone.fullName(name)
	question := DevDnsQuestion{Name: zone.owner(asked), Type: zone.typeNumber(kind)}
	answers := zone.answers(question, asked)
	return DevDnsResponse{Status: zone.status(answers), Answer: answers}
}

func (zone DevDnsZone) fullName(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), zone.rootDot()) + zone.rootDot()
}

func (zone DevDnsZone) owner(asked string) string {
	return regexp.MustCompile(`^[^.]+\._domainkey\.`).ReplaceAllString(asked, zone.anySelector()+zone.dkimLabel())
}

func (zone DevDnsZone) typeNumber(kind string) int {
	numbers := map[string]int{"MX": zone.mxType(), "TXT": zone.txtType()}
	return numbers[strings.ToUpper(strings.TrimSpace(kind))]
}

func (zone DevDnsZone) answers(question DevDnsQuestion, asked string) []DevDnsAnswer {
	answers := []DevDnsAnswer{}
	for _, record := range zone.records() {
		answers = append(answers, zone.answerIfAsked(record, question, asked)...)
	}
	return answers
}

func (zone DevDnsZone) answerIfAsked(record DevDnsRecord, question DevDnsQuestion, asked string) []DevDnsAnswer {
	if record.Question != question {
		return nil
	}
	return []DevDnsAnswer{{Name: asked, Type: record.Question.Type, TTL: zone.timeToLive(), Data: record.Data}}
}

func (zone DevDnsZone) status(answers []DevDnsAnswer) int {
	if len(answers) > 0 {
		return zone.noError()
	}
	return zone.nameMissing()
}

func (zone DevDnsZone) records() []DevDnsRecord {
	records := []DevDnsRecord{}
	for _, domain := range zone.Settings.Domains {
		records = append(records, zone.domainRecords(domain+zone.rootDot())...)
	}
	return records
}

func (zone DevDnsZone) domainRecords(domain string) []DevDnsRecord {
	return []DevDnsRecord{
		{Question: DevDnsQuestion{Name: domain, Type: zone.mxType()}, Data: zone.MailExchange()},
		{Question: DevDnsQuestion{Name: domain, Type: zone.txtType()}, Data: zone.SpfText()},
		{Question: DevDnsQuestion{Name: zone.anySelector() + zone.dkimLabel() + domain, Type: zone.txtType()}, Data: zone.DkimText()},
		{Question: DevDnsQuestion{Name: zone.dmarcLabel() + domain, Type: zone.txtType()}, Data: zone.DmarcText()},
	}
}

func (zone DevDnsZone) MailExchange() string {
	return "10 " + zone.Settings.MailHost + zone.rootDot()
}

func (zone DevDnsZone) SpfText() string {
	return `"v=spf1 ` + zone.Settings.Spf + ` -all"`
}

func (DevDnsZone) DkimText() string {
	return `"v=DKIM1; k=rsa; p=bWVzYWlsLWRldi1kbnMtcGxhY2Vob2xkZXIta2V5"`
}

func (DevDnsZone) DmarcText() string {
	return `"v=DMARC1; p=reject"`
}

func (DevDnsZone) anySelector() string {
	return "*"
}

func (DevDnsZone) dkimLabel() string {
	return "._domainkey."
}

func (DevDnsZone) dmarcLabel() string {
	return "_dmarc."
}

func (DevDnsZone) rootDot() string {
	return "."
}

func (DevDnsZone) mxType() int {
	return 15
}

func (DevDnsZone) txtType() int {
	return 16
}

func (DevDnsZone) timeToLive() int {
	return 60
}

func (DevDnsZone) noError() int {
	return 0
}

func (DevDnsZone) nameMissing() int {
	return 3
}
