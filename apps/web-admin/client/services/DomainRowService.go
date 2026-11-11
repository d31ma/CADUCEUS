// @Service
type DomainRowService struct {
	// @Needs
	records *DomainRecordService
}

func (rows *DomainRowService) Row(domain Domain) DomainRow {
	state := rows.stateLook(domain.State)
	return DomainRow{
		DomainId:    domain.DomainId,
		Name:        domain.Name,
		RowClass:    state.RowClass,
		StateText:   state.Text,
		StateClass:  state.TagClass,
		StateIcon:   state.IconClass,
		StateNote:   state.Note,
		HasNote:     state.Note != "",
		Marks:       rows.records.Marks(domain.Dns),
		MarksLabel:  rows.records.MarksLabel(domain.Dns),
		ActionText:  state.ActionText,
		ActionClass: state.ActionClass,
		ActionLabel: state.ActionPrefix + domain.Name,
	}
}

func (rows *DomainRowService) View(domain Domain) DomainView {
	state := rows.stateLook(domain.State)
	return DomainView{
		StateText:  state.Text,
		StateClass: state.TagClass,
		StateIcon:  state.IconClass,
		Pending:    state.Pending,
		Waiting:    rows.records.Waiting(domain.Dns),
		Status:     rows.records.Status(domain.Dns),
		Records:    rows.records.Records(domain),
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
	return map[string]DomainStateLook{"pending": rows.pendingLook(), "active": rows.activeLook(), "disabled": rows.disabledLook()}
}

func (rows *DomainRowService) pendingLook() DomainStateLook {
	return DomainStateLook{
		Text:         "Pending",
		TagClass:     "ms-tag ms-tag--warn",
		IconClass:    "ms-icon ms-i-clock",
		Note:         "Mail is not accepted yet: MX and DKIM must be OK.",
		ActionText:   "Set up DNS",
		ActionClass:  "ms-btn ms-btn--sm ms-btn--primary",
		ActionPrefix: "Set up DNS for ",
		Pending:      true,
	}
}

func (rows *DomainRowService) activeLook() DomainStateLook {
	return DomainStateLook{
		Text:         "Active",
		TagClass:     "ms-tag ms-tag--ok",
		IconClass:    "ms-icon ms-i-check",
		ActionText:   rows.manage(),
		ActionClass:  rows.smallButton(),
		ActionPrefix: rows.managePrefix(),
	}
}

func (rows *DomainRowService) disabledLook() DomainStateLook {
	return DomainStateLook{
		Text:         "Disabled",
		TagClass:     rows.outlineTag(),
		IconClass:    rows.offIcon(),
		Note:         "Mail to this domain is refused. Messages are kept.",
		RowClass:     "is-muted",
		ActionText:   rows.manage(),
		ActionClass:  rows.smallButton(),
		ActionPrefix: rows.managePrefix(),
	}
}

func (rows *DomainRowService) unknownLook() DomainStateLook {
	return DomainStateLook{
		Text:         "Unknown",
		TagClass:     rows.outlineTag(),
		IconClass:    rows.offIcon(),
		ActionText:   rows.manage(),
		ActionClass:  rows.smallButton(),
		ActionPrefix: rows.managePrefix(),
	}
}

func (rows *DomainRowService) manage() string {
	return "Manage"
}

func (rows *DomainRowService) managePrefix() string {
	return "DNS and settings for "
}

func (rows *DomainRowService) smallButton() string {
	return "ms-btn ms-btn--sm"
}

func (rows *DomainRowService) outlineTag() string {
	return "ms-tag ms-tag--outline"
}

func (rows *DomainRowService) offIcon() string {
	return "ms-icon ms-i-minus-circle"
}
