package notify

import "slices"

type SettingsCheck struct{}

func (check SettingsCheck) Check(afterMinutes int, mutedConversationIds []string) error {
	err := check.refuseWhen(!check.delayAllowed(afterMinutes), InvalidSettings{Field: check.delayField(), Reason: "must be 0 or between 15 and 1440"})
	if err != nil {
		return err
	}
	err = check.refuseWhen(len(mutedConversationIds) > check.mostMuted(), InvalidSettings{Field: check.mutedField(), Reason: "may hold at most 500 ids"})
	if err != nil {
		return err
	}
	return check.refuseWhen(slices.ContainsFunc(mutedConversationIds, check.badId), InvalidSettings{Field: check.mutedField(), Reason: "must hold ids of 1 to 64 characters"})
}

func (SettingsCheck) delayField() string {
	return "afterMinutes"
}

func (SettingsCheck) mutedField() string {
	return "mutedConversationIds"
}

func (SettingsCheck) shortestDelay() int {
	return 15
}

func (SettingsCheck) longestDelay() int {
	return 1440
}

func (SettingsCheck) mostMuted() int {
	return 500
}

func (SettingsCheck) longestId() int {
	return 64
}

func (check SettingsCheck) delayAllowed(afterMinutes int) bool {
	return afterMinutes == 0 || (afterMinutes >= check.shortestDelay() && afterMinutes <= check.longestDelay())
}

func (check SettingsCheck) badId(id string) bool {
	return id == "" || len(id) > check.longestId()
}

func (SettingsCheck) refuseWhen(refused bool, failure InvalidSettings) error {
	if refused {
		return failure
	}
	return nil
}
