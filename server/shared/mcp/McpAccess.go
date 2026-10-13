import (
	"sort"
	"strings"
)

// McpAccess keeps each mailbox's MCP configuration and its one active
// capability, which is handed out once and stored only as a hash.
type McpAccess struct {
	Tokens *McpAccessTokensHelper
	Users  *UsersHelper
	Fylo   *FyloHelper
}

// Config answers the owner's MCP configuration and token state, or nil when
// the owner has no user record.
func (access McpAccess) Config(ownerEmail string) (map[string]any, error) {
	_, user, err := access.Users.FindByEmail(ownerEmail)
	if err != nil || user == nil {
		return nil, err
	}
	records, err := access.Tokens.ListForOwner(ownerEmail)
	if err != nil {
		return nil, err
	}
	config := McpPreferences{}.Normalized(user["mcp"], access.domains(user))
	active := access.active(records)
	config["hasToken"] = active != nil
	config["tokenCreatedAt"] = ""
	if active != nil {
		config["tokenCreatedAt"] = RecordValues{}.Or(active["createdAt"], "")
	}
	config["endpoint"] = "/mcp"
	config["protocolVersions"] = []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05", "2024-10-07"}
	return config, nil
}

// UpdateConfig merges a patch into the owner's configuration and answers the
// new configuration, nil when the owner has no user record, or an
// *McpPreferenceError when the result is refused.
func (access McpAccess) UpdateConfig(ownerEmail string, patch any) (map[string]any, error) {
	documentId, user, err := access.Users.FindByEmail(ownerEmail)
	if err != nil || documentId == "" || user == nil {
		return nil, err
	}
	merged := patch
	if object, isObject := (RecordValues{}).Object(patch); isObject {
		current := McpPreferences{}.Normalized(user["mcp"], access.domains(user))
		for key, value := range object {
			current[key] = value
		}
		merged = current
	}
	preferences, refusal := McpPreferenceRules{}.Validated(merged, access.domains(user))
	if refusal != nil {
		return nil, refusal
	}
	if err := access.Users.UpdateMcpPreferences(documentId, preferences); err != nil {
		return nil, err
	}
	return access.Config(ownerEmail)
}

// Rotate revokes the owner's active capability, keeps at most four older
// records, and answers a new capability: the only time its text is seen.
func (access McpAccess) Rotate(ownerEmail string) (map[string]any, error) {
	documentId, user, err := access.Users.FindByEmail(ownerEmail)
	if err != nil || documentId == "" || user == nil {
		return nil, err
	}
	domain := access.domainOf(ownerEmail)
	if domain == "" {
		return nil, nil
	}
	now := EpochTime{}.NowText()
	existing, err := access.Tokens.ListForOwner(ownerEmail)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(existing, func(left, right int) bool {
		return TextOrder{}.Compare(TextValue{}.Of(existing[right]["createdAt"]), TextValue{}.Of(existing[left]["createdAt"])) < 0
	})
	if _, err := access.revokeActive(existing, now); err != nil {
		return nil, err
	}
	for index, stale := range existing {
		if index >= access.maximumOldTokens()-1 {
			if err := access.Tokens.Delete(RecordValues{}.Text(stale["docId"])); err != nil {
				return nil, err
			}
		}
	}
	token := CapabilityToken{Prefix: access.prefix()}.Create(domain)
	record := map[string]any{
		"id":         "mcp-token-" + access.randomText(16),
		"ownerEmail": strings.ToLower(ownerEmail),
		"domain":     domain,
		"tokenHash":  CapabilityToken{}.Hash(token),
		"active":     true,
		"createdAt":  now,
	}
	if _, err := access.Tokens.Put(record); err != nil {
		return nil, err
	}
	return map[string]any{"token": token, "tokenId": record["id"], "createdAt": now}, nil
}

// Revoke revokes the owner's active capabilities and reports whether there
// was one.
func (access McpAccess) Revoke(ownerEmail string) (bool, error) {
	records, err := access.Tokens.ListForOwner(ownerEmail)
	if err != nil {
		return false, err
	}
	revoked, err := access.revokeActive(records, EpochTime{}.NowText())
	return revoked > 0, err
}

// Authenticate answers the principal of an MCP bearer token, or nil. The
// token's domain only says where to look the hash up; the stored record and
// its owner's current preferences decide the scope.
func (access McpAccess) Authenticate(token string) (*McpPrincipal, error) {
	value := Security{}.Trim(token)
	parsed, valid := CapabilityToken{Prefix: access.prefix()}.Parse(value, 420)
	if !valid || len(value) > 420 {
		return nil, nil
	}
	previous := access.Fylo.ActForDomain(parsed.Domain)
	defer access.Fylo.Restore(previous)
	candidates, err := access.Tokens.FindByHash(parsed.Hash)
	if err != nil {
		return nil, err
	}
	var record map[string]any
	for _, candidate := range candidates {
		if access.isActive(candidate) && candidate["domain"] == parsed.Domain && (CapabilityToken{}).EqualHash(TextValue{}.Of(RecordValues{}.Or(candidate["tokenHash"], "")), parsed.Hash) {
			record = candidate
			break
		}
	}
	if record == nil {
		return nil, nil
	}
	return access.principal(record, parsed.Domain)
}

func (access McpAccess) principal(record map[string]any, primaryDomain string) (*McpPrincipal, error) {
	_, user, err := access.Users.FindByEmail(RecordValues{}.Text(record["ownerEmail"]))
	if err != nil || user == nil {
		return nil, err
	}
	preferences := McpPreferences{}.Normalized(user["mcp"], access.domains(user))
	allowed, _ := preferences["allowedDomains"].([]string)
	if preferences["enabled"] != true || len(allowed) == 0 {
		return nil, nil
	}
	if err := access.Tokens.Update(RecordValues{}.Text(record["docId"]), map[string]any{"lastUsedAt": EpochTime{}.NowText()}); err != nil {
		return nil, err
	}
	excluded := map[string][]string{}
	folders, _ := preferences["excludedFolders"].(map[string]any)
	for domain, list := range folders {
		excluded[domain], _ = list.([]string)
	}
	return &McpPrincipal{
		OwnerEmail:      RecordValues{}.Text(record["ownerEmail"]),
		TokenId:         RecordValues{}.Text(record["id"]),
		PrimaryDomain:   primaryDomain,
		AccessLevel:     RecordValues{}.Text(preferences["accessLevel"]),
		AllowedDomains:  allowed,
		ExcludedFolders: excluded,
	}, nil
}

func (access McpAccess) revokeActive(records []map[string]any, now string) (int, error) {
	revoked := 0
	for _, record := range records {
		if access.isActive(record) {
			if err := access.Tokens.Update(RecordValues{}.Text(record["docId"]), map[string]any{"active": false, "revokedAt": now}); err != nil {
				return revoked, err
			}
			revoked++
		}
	}
	return revoked, nil
}

func (access McpAccess) active(records []map[string]any) map[string]any {
	for _, record := range records {
		if access.isActive(record) {
			return record
		}
	}
	return nil
}

func (McpAccess) isActive(record map[string]any) bool {
	return record["active"] == true && !(RecordValues{}).Truthy(record["revokedAt"])
}

func (McpAccess) domains(user map[string]any) []string {
	domains, _ := user["domains"].([]string)
	return domains
}

func (McpAccess) domainOf(email string) string {
	normalized := Security{}.NormalizeEmailAddress(email)
	if parts := strings.SplitN(normalized, "@", 2); len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// randomText answers size random bytes as base64url, as
// Security.randomText(size, 'base64url') did.
func (McpAccess) randomText(size int) string {
	return CapabilityToken{}.Random(size)
}

func (McpAccess) prefix() string { return "mcp1." }

func (McpAccess) maximumOldTokens() int { return 5 }
