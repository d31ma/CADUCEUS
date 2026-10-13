import "slices"

// ReplyUtils builds the reply target and RFC reference chain for the
// composer and for quick replies.
type ReplyUtils struct{}

func (ReplyUtils) Target(replyTo []string, sender string) string {
	return ReplyUtils{}.target(replyTo, sender)
}

func (ReplyUtils) References(references []string, parent string) []string {
	chain := append([]string{}, references...)
	return ReplyUtils{}.withParent(chain, parent)
}

func (ReplyUtils) target(replyTo []string, sender string) string {
	if len(replyTo) > 0 && replyTo[0] != "" {
		return replyTo[0]
	}
	return sender
}

func (ReplyUtils) withParent(chain []string, parent string) []string {
	if slices.Contains(chain, parent) {
		return chain
	}
	return append(ReplyUtils{}.capped(chain), parent)
}

func (ReplyUtils) maximumReferences() int { return 100 }

func (ReplyUtils) capped(chain []string) []string {
	if len(chain) >= (ReplyUtils{}).maximumReferences() {
		return append([]string{chain[0]}, chain[len(chain)-98:]...)
	}
	return chain
}
