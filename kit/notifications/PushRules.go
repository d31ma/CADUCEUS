package notifications

import "time"

type PushRules struct {
	decisions PushDecisions
}

func (rules PushRules) Decide(state string, doNotDisturbUntil string, now time.Time) string {
	byState := map[string]string{
		rules.active():       rules.decisions.SkipActive(),
		rules.doNotDisturb(): rules.doNotDisturbDecision(doNotDisturbUntil, now),
	}
	return rules.orSend(byState, state)
}

func (PushRules) active() string {
	return "active"
}

func (PushRules) doNotDisturb() string {
	return "dnd"
}

func (rules PushRules) doNotDisturbDecision(doNotDisturbUntil string, now time.Time) string {
	byEnded := map[bool]string{
		true:  rules.decisions.Send(),
		false: rules.decisions.SkipDoNotDisturb(),
	}
	return byEnded[rules.ended(doNotDisturbUntil, now)]
}

func (PushRules) ended(doNotDisturbUntil string, now time.Time) bool {
	end, err := time.Parse(time.RFC3339, doNotDisturbUntil)
	return err == nil && !end.After(now)
}

func (rules PushRules) orSend(byState map[string]string, state string) string {
	decision, found := byState[state]
	if !found {
		return rules.decisions.Send()
	}
	return decision
}
