package main

import (
	"net/http"
	"path"
)

type EntryRule struct {
}

func (rule EntryRule) For(event HttpEvent) SlotEntry {
	return rule.entry(rule.eventPush(event))
}

func (EntryRule) entry(eventPush bool) SlotEntry {
	if eventPush {
		return ImmediateEntry{}
	}
	return QueuedEntry{}
}

func (rule EntryRule) eventPush(event HttpEvent) bool {
	matched, _ := path.Match(rule.eventsPath(), event.RawPath)
	return matched && event.RequestContext.Http.Method == http.MethodPost
}

func (EntryRule) eventsPath() string {
	return "/api/v1/*/events"
}
