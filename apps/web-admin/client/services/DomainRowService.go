// @Service
type DomainRowService struct {
	// @Needs
	records *DomainRecordService
}

func (rows *DomainRowService) Row(domain Domain) DomainRow {
	state := rows.stateLook(domain.State)
	return DomainRow{
		DomainId:      domain.DomainId,
		Name:          domain.Name,
		Tone:          state.Tone,
		StateText:     state.Text,
		StateColor:    state.Color,
		StateVariant:  state.Variant,
		StateIcon:     state.Icon,
		StateNote:     state.Note,
		HasNote:       state.Note != "",
		Marks:         rows.records.Marks(domain.Dns),
		MarksLabel:    rows.records.MarksLabel(domain.Dns),
		ActionText:    state.ActionText,
		ActionVariant: state.ActionVariant,
		ActionLabel:   state.ActionPrefix + domain.Name,
		ActionOff:     state.Locked,
	}
}

func (rows *DomainRowService) View(domain Domain) DomainView {
	state := rows.stateLook(domain.State)
	return DomainView{
		StateText:    state.Text,
		StateColor:   state.Color,
		StateVariant: state.Variant,
		StateIcon:    state.Icon,
		Pending:      state.Pending,
		Waiting:      rows.records.Waiting(domain.Dns),
		Status:       rows.records.Status(domain.Dns),
		Records:      rows.records.Records(domain),
	}
}

func (rows *DomainRowService) stateLook(state string) DomainStateLook {
	look, known := rows.looks()[state]
	if !known {
		return rows.unknownLook()
	}
	return look
}

func (rows *DomainRowService) looks() map[string]DomainStateLook {
	return map[string]DomainStateLook{"pending": rows.pendingLook(), "active": rows.activeLook(), "disabled": rows.disabledLook(), "removing": rows.removingLook()}
}

func (rows *DomainRowService) pendingLook() DomainStateLook {
	return DomainStateLook{
		Text:          "Pending",
		Color:         "warning",
		Icon:          "clock",
		Note:          "Mail is not accepted yet: MX and DKIM must be OK.",
		ActionText:    "Set up DNS",
		ActionVariant: "flat",
		ActionPrefix:  "Set up DNS for ",
		Pending:       true,
	}
}

func (rows *DomainRowService) activeLook() DomainStateLook {
	return DomainStateLook{
		Text:          "Active",
		Color:         "success",
		Icon:          "check",
		ActionText:    rows.manage(),
		ActionVariant: rows.outlined(),
		ActionPrefix:  rows.managePrefix(),
	}
}

func (rows *DomainRowService) disabledLook() DomainStateLook {
	return DomainStateLook{
		Text:          "Disabled",
		Variant:       rows.outlined(),
		Icon:          rows.offIcon(),
		Note:          "Mail to this domain is refused. Messages are kept.",
		Tone:          rows.muted(),
		ActionText:    rows.manage(),
		ActionVariant: rows.outlined(),
		ActionPrefix:  rows.managePrefix(),
	}
}

func (rows *DomainRowService) removingLook() DomainStateLook {
	return DomainStateLook{
		Text:          "Removing",
		Color:         "error",
		Icon:          "trash",
		Note:          "Mail to this domain is refused. Mesail removes its members and addresses in the background.",
		Tone:          rows.muted(),
		ActionText:    rows.manage(),
		ActionVariant: rows.outlined(),
		ActionPrefix:  "Manage is off while Mesail removes ",
		Locked:        true,
	}
}

func (rows *DomainRowService) unknownLook() DomainStateLook {
	return DomainStateLook{
		Text:          "Unknown",
		Variant:       rows.outlined(),
		Icon:          rows.offIcon(),
		ActionText:    rows.manage(),
		ActionVariant: rows.outlined(),
		ActionPrefix:  rows.managePrefix(),
	}
}

func (rows *DomainRowService) manage() string {
	return "Manage"
}

func (rows *DomainRowService) managePrefix() string {
	return "DNS and settings for "
}

func (rows *DomainRowService) muted() string {
	return "muted"
}

func (rows *DomainRowService) outlined() string {
	return "outlined"
}

func (rows *DomainRowService) offIcon() string {
	return "minus-circle"
}
