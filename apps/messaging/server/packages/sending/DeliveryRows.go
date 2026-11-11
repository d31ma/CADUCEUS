package sending

import "mesail.dev/packages/core"

type DeliveryRows struct{}

func (rows DeliveryRows) email(all []core.DeliveryRecord) []core.DeliveryRecord {
	kept := []core.DeliveryRecord{}
	for _, row := range all {
		kept = append(kept, rows.ifChannel(row, core.DeliveryChannels{}.Email())...)
	}
	return kept
}

func (rows DeliveryRows) addresses(emailRows []core.DeliveryRecord, role string) []string {
	addresses := []string{}
	for _, row := range emailRows {
		addresses = append(addresses, rows.ifRole(row, role)...)
	}
	return addresses
}

func (rows DeliveryRows) chatOnlyMembers(all []core.DeliveryRecord) []string {
	emailed := rows.recipients(rows.email(all))
	addresses := []string{}
	for _, row := range all {
		addresses = append(addresses, rows.ifChatOnlyMember(row, emailed)...)
	}
	return addresses
}

func (DeliveryRows) recipients(emailRows []core.DeliveryRecord) map[string]bool {
	emailed := map[string]bool{}
	for _, row := range emailRows {
		emailed[row.Recipient] = true
	}
	return emailed
}

func (DeliveryRows) ifChannel(row core.DeliveryRecord, channel string) []core.DeliveryRecord {
	if row.Channel != channel {
		return nil
	}
	return []core.DeliveryRecord{row}
}

func (DeliveryRows) ifRole(row core.DeliveryRecord, role string) []string {
	if row.Role != role {
		return nil
	}
	return []string{row.Recipient}
}

func (DeliveryRows) ifChatOnlyMember(row core.DeliveryRecord, emailed map[string]bool) []string {
	if row.Channel != (core.DeliveryChannels{}).Chat() || row.MemberId == "" || emailed[row.Recipient] {
		return nil
	}
	return []string{row.Recipient}
}
