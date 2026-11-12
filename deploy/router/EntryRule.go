package main

import (
	"net/http"
	"path"
	"slices"
)

type EntryRule struct {
}

func (rule EntryRule) For(event HttpEvent) SlotEntry {
	return rule.entry(rule.eventPush(event), rule.background(event.RawPath))
}

func (EntryRule) entry(eventPush bool, background bool) SlotEntry {
	if eventPush {
		return ImmediateEntry{}
	}
	return QueuedEntry{Background: background}
}

func (rule EntryRule) eventPush(event HttpEvent) bool {
	return rule.matches(rule.eventsPath(), event.RawPath) && event.RequestContext.Http.Method == http.MethodPost
}

func (rule EntryRule) background(rawPath string) bool {
	return slices.ContainsFunc(rule.backgroundPaths(), func(pattern string) bool { return rule.matches(pattern, rawPath) })
}

func (EntryRule) matches(pattern string, rawPath string) bool {
	matched, _ := path.Match(pattern, rawPath)
	return matched
}

func (EntryRule) eventsPath() string {
	return "/api/v1/*/events"
}

func (EntryRule) backgroundPaths() []string {
	return []string{"/api/v1/*/outbox/relay", "/api/v1/*/outbox/dead/requeue", "/api/v1/*/jobs/*"}
}
