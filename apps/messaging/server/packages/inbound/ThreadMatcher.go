package inbound

import (
	"slices"
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type ThreadMatcher struct {
	threadKeys core.ThreadKeyStore
}

func NewThreadMatcher(threadKeys core.ThreadKeyStore) ThreadMatcher {
	return ThreadMatcher{threadKeys: threadKeys}
}

func (matcher ThreadMatcher) Find(domainId string, headers events.EmailHeaders) (core.ThreadKey, bool, error) {
	return matcher.first(domainId, matcher.candidates(headers))
}

func (matcher ThreadMatcher) candidates(headers events.EmailHeaders) []string {
	candidates := []string{headers.InReplyTo}
	for index := len(headers.References) - 1; index >= 0; index-- {
		candidates = append(candidates, headers.References[index])
	}
	candidates = slices.DeleteFunc(candidates, matcher.blank)
	return candidates[:min(len(candidates), matcher.maximumLookups())]
}

func (ThreadMatcher) blank(messageId string) bool {
	return strings.TrimSpace(messageId) == ""
}

func (ThreadMatcher) maximumLookups() int {
	return 20
}

func (matcher ThreadMatcher) first(domainId string, candidates []string) (core.ThreadKey, bool, error) {
	for _, candidate := range candidates {
		key, found, err := matcher.threadKeys.ByKey(domainId, MessageIdKeys{}.Of(candidate))
		if err != nil {
			return core.ThreadKey{}, false, Unavailability{}.Of(err)
		}
		if found {
			return key, true, nil
		}
	}
	return core.ThreadKey{}, false, nil
}
