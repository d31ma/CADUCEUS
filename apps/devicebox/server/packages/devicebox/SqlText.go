package devicebox

import (
	"errors"
	"regexp"
)

type SqlText struct{}

func (text SqlText) WaitingFor(slips string, deviceKey string, afterUs int64, limit int) (Statement, error) {
	err := text.safe(slips, deviceKey)
	sql := "SELECT envelopeId, senderDeviceId, senderPersonId, kind, storedAt, storedUs, bodyId FROM " + slips +
		" WHERE waiting." + deviceKey + " = true AND storedUs > $1 LIMIT $2"
	return Statement{Sql: sql, Parameters: []any{afterUs, limit}}, err
}

func (text SqlText) SlipByEnvelope(slips string, envelopeId string) (Statement, error) {
	return text.firstWhere(slips, "envelopeId", envelopeId)
}

func (text SqlText) Bucket(counts string, bucket string) (Statement, error) {
	return text.firstWhere(counts, "bucket", bucket)
}

func (text SqlText) RegistryRecord(registry string, name string) (Statement, error) {
	return text.firstWhere(registry, "name", name)
}

func (text SqlText) Registry(registry string) (Statement, error) {
	err := text.safe(registry, "")
	return Statement{Sql: "SELECT name, domainId FROM " + registry + " LIMIT $1", Parameters: []any{text.registryLimit()}}, err
}

func (text SqlText) ExpiredSlips(slips string, nowUs int64, limit int) (Statement, error) {
	err := text.safe(slips, "")
	return Statement{Sql: "SELECT * FROM " + slips + " WHERE expiresUs < $1 LIMIT $2", Parameters: []any{nowUs, limit}}, err
}

func (text SqlText) ExpiredBodies(bodies string, nowUs int64, limit int) (Statement, error) {
	err := text.safe(bodies, "")
	return Statement{Sql: "SELECT envelopeId FROM " + bodies + " WHERE expiresUs < $1 LIMIT $2", Parameters: []any{nowUs, limit}}, err
}

func (text SqlText) Outbox(bodies string, flag string, limit int) (Statement, error) {
	statement, err := text.AllOutbox(bodies, flag)
	if err != nil {
		return Statement{}, err
	}
	return Statement{Sql: statement.Sql + " LIMIT $1", Parameters: []any{limit}}, nil
}

func (text SqlText) AllOutbox(bodies string, flag string) (Statement, error) {
	err := text.knownFlag(flag)
	if err != nil {
		return Statement{}, err
	}
	sql := "SELECT pendingEvents, deadEvents, outboxPending, outboxDead FROM " + bodies + " WHERE " + flag + " = true"
	return Statement{Sql: sql, Parameters: []any{}}, text.safe(bodies, "")
}

func (text SqlText) firstWhere(collection string, field string, value string) (Statement, error) {
	err := text.safe(collection, "")
	return Statement{Sql: "SELECT * FROM " + collection + " WHERE " + field + " = $1 LIMIT 1", Parameters: []any{value}}, err
}

func (text SqlText) safe(collection string, deviceKey string) error {
	if !regexp.MustCompile(`^[a-z0-9-]{1,128}$`).MatchString(collection) {
		return Unavailable{}.StoreDown(errors.New("a collection name is not safe for SQL"))
	}
	return text.safeKey(deviceKey)
}

func (SqlText) safeKey(deviceKey string) error {
	if deviceKey != "" && !(DeviceKey{}).Valid(deviceKey) {
		return Unavailable{}.StoreDown(errors.New("a device key is not safe for SQL"))
	}
	return nil
}

func (SqlText) knownFlag(flag string) error {
	if flag != "outboxWaiting" && flag != "outboxHasDead" {
		return Unavailable{}.StoreDown(errors.New("an outbox flag is not known"))
	}
	return nil
}

func (SqlText) registryLimit() int {
	return 1000
}
