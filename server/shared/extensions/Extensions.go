import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

// Extensions inspects sideloaded extension packages (plugins and themes),
// builds the owner's trusted record once a digest was trusted, and presents
// records to the settings screens.
type Extensions struct{}

// Inspect parses and normalizes one package without trusting it, answering
// {manifest, source, digest} or the reason the package is refused.
func (extensions Extensions) Inspect(bundle any) (map[string]any, string) {
	object, isObject := bundle.(map[string]any)
	if !isObject || extensions.unknownKeys(object, []string{"manifest", "source"}) {
		return nil, "Extension package must contain only manifest and source"
	}
	manifest, problem := extensions.manifest(object["manifest"])
	if problem != "" {
		return nil, problem
	}
	source, _ := object["source"].(string)
	if len(source) > extensions.maximumSourceBytes() {
		return nil, "Extension source exceeds 128 KiB"
	}
	if manifest["kind"] == "plugin" && (Security{}).Trim(source) == "" {
		return nil, "Plugin source is required"
	}
	if manifest["kind"] == "theme" && source != "" {
		return nil, "Themes cannot contain executable source"
	}
	normalized := map[string]any{"manifest": manifest, "source": source}
	digest := sha256.Sum256([]byte(StableJson{}.Text(normalized)))
	return map[string]any{"manifest": manifest, "source": source, "digest": hex.EncodeToString(digest[:])}, ""
}

// TrustedRecord answers the owner's durable record of an inspected package.
func (Extensions) TrustedRecord(owner string, inspected map[string]any) map[string]any {
	timestamp := EpochTime{}.NowText()
	normalizedOwner := strings.ToLower(owner)
	manifest, _ := inspected["manifest"].(map[string]any)
	domain := ""
	if parts := strings.SplitN(normalizedOwner, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}
	return map[string]any{
		"id":           "extension-" + Security{}.RandomUuid(),
		"extensionId":  manifest["id"],
		"owner":        normalizedOwner,
		"domain":       domain,
		"kind":         manifest["kind"],
		"displayName":  manifest["displayName"],
		"version":      manifest["version"],
		"digest":       inspected["digest"],
		"manifestJson": TextValue{}.Json(manifest),
		"source":       inspected["source"],
		"enabled":      false,
		"trustedAt":    timestamp,
		"installedAt":  timestamp,
		"updatedAt":    timestamp,
	}
}

// Present answers the settings view of a record; runtime adds the source.
// Fields the record lacks stay out, as JSON.stringify left out undefined.
func (extensions Extensions) Present(record map[string]any, runtime bool) (map[string]any, error) {
	manifest, hasManifest := record["manifest"].(map[string]any)
	if !hasManifest {
		var parsed any
		if json.Unmarshal([]byte(RecordValues{}.Text(record["manifestJson"])), &parsed) != nil {
			return nil, errors.New("Stored extension manifest is invalid")
		}
		manifest, _ = parsed.(map[string]any)
	}
	enabled, _ := record["enabled"].(bool)
	result := map[string]any{"enabled": enabled, "manifest": manifest}
	for field, fallback := range map[string]string{"id": "extensionId", "displayName": "displayName", "version": "version", "kind": "kind"} {
		if value := (RecordValues{}).Coalesce(record[fallback], manifest[field]); value != nil {
			result[field] = value
		}
	}
	for _, field := range []string{"digest", "trustedAt", "installedAt", "updatedAt"} {
		if value, present := record[field]; present {
			result[field] = value
		}
	}
	if runtime {
		result["source"] = RecordValues{}.Coalesce(record["source"], "")
	}
	return result, nil
}

func (extensions Extensions) manifest(value any) (map[string]any, string) {
	allowed := []string{"schemaVersion", "id", "displayName", "description", "version", "kind", "engine", "permissions", "contributions", "theme"}
	manifest, isObject := value.(map[string]any)
	if !isObject || extensions.unknownKeys(manifest, allowed) {
		return nil, "Extension manifest has unknown or invalid fields"
	}
	if version, _ := manifest["schemaVersion"].(float64); version != 1 {
		return nil, "Extension manifest schemaVersion must be 1"
	}
	id, isText := manifest["id"].(string)
	if !isText || !regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z0-9][a-z0-9-]*){1,5}$`).MatchString(id) || (TextValue{}).Length(id) > 128 {
		return nil, "Extension id is invalid"
	}
	if !extensions.boundedText(manifest["displayName"], 80, true) || !extensions.boundedText(RecordValues{}.Coalesce(manifest["description"], ""), 500, false) {
		return nil, "Extension display metadata is invalid"
	}
	version, isText := manifest["version"].(string)
	if !isText || !regexp.MustCompile(`^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$`).MatchString(version) {
		return nil, "Extension version must be semantic"
	}
	kind := manifest["kind"]
	if kind != "plugin" && kind != "theme" {
		return nil, "Extension kind is invalid"
	}
	engine, isObject := manifest["engine"].(map[string]any)
	if !isObject || extensions.unknownKeys(engine, []string{"caduceus"}) || engine["caduceus"] != "26" {
		return nil, "Extension engine must target CADUCEUS 26"
	}
	permissions, valid := extensions.permissions(manifest["permissions"])
	if !valid {
		return nil, "Extension permissions are invalid"
	}
	if kind == "theme" {
		return extensions.themeManifest(manifest, permissions)
	}
	return extensions.pluginManifest(manifest, permissions)
}

func (extensions Extensions) permissions(value any) ([]string, bool) {
	list, isList := value.([]any)
	allowed := []string{"compose:read", "compose:write", "mail:metadata", "ui:notify"}
	if !isList || len(list) > len(allowed) {
		return nil, false
	}
	permissions := []string{}
	for _, item := range list {
		permission, isText := item.(string)
		if !isText || !(PreferenceChoices{}).Includes(allowed, permission) || (PreferenceChoices{}).Includes(permissions, permission) {
			return nil, false
		}
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	return permissions, true
}

func (extensions Extensions) pluginManifest(manifest map[string]any, permissions []string) (map[string]any, string) {
	if _, declared := manifest["theme"]; declared {
		return nil, "Plugins cannot declare theme tokens"
	}
	list, isList := manifest["contributions"].([]any)
	if !isList || len(list) == 0 || len(list) > extensions.maximumContributions() {
		return nil, "Plugin contributions are required and bounded"
	}
	slots := []string{"app-toolbar", "composer-toolbar", "message-toolbar"}
	seen := []string{}
	contributions := []any{}
	for _, item := range list {
		contribution, isObject := item.(map[string]any)
		if !isObject || extensions.unknownKeys(contribution, []string{"id", "label", "slot"}) {
			return nil, "Plugin contribution is invalid"
		}
		id, isText := contribution["id"].(string)
		slot, _ := contribution["slot"].(string)
		if !isText || !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(id) || (PreferenceChoices{}).Includes(seen, id) || !extensions.boundedText(contribution["label"], 60, true) || !(PreferenceChoices{}).Includes(slots, slot) {
			return nil, "Plugin contribution is invalid"
		}
		seen = append(seen, id)
		contributions = append(contributions, map[string]any{"id": id, "label": Security{}.Trim(contribution["label"].(string)), "slot": slot})
	}
	normalized := extensions.common(manifest, "plugin")
	normalized["permissions"] = permissions
	normalized["contributions"] = contributions
	return normalized, ""
}

func (extensions Extensions) themeManifest(manifest map[string]any, permissions []string) (map[string]any, string) {
	if _, declared := manifest["contributions"]; len(permissions) > 0 || declared {
		return nil, "Themes cannot request permissions or commands"
	}
	theme, isObject := manifest["theme"].(map[string]any)
	if !isObject || extensions.unknownKeys(theme, []string{"mode", "tokens"}) || !(PreferenceChoices{}).Includes([]string{"light", "dark", "both"}, RecordValues{}.Text(theme["mode"])) {
		return nil, "Theme definition is invalid"
	}
	tokens, isObject := theme["tokens"].(map[string]any)
	if !isObject {
		return nil, "Theme definition is invalid"
	}
	allowed := []string{"danger", "on-primary", "primary", "primary-container", "surface", "surface-container", "surface-container-high", "text", "text-subtle"}
	if len(tokens) < 3 || len(tokens) > len(allowed) {
		return nil, "Theme tokens must be allowlisted hexadecimal colors"
	}
	normalizedTokens := map[string]any{}
	for key, value := range tokens {
		color, isText := value.(string)
		if !(PreferenceChoices{}).Includes(allowed, key) || !isText || !regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`).MatchString(color) {
			return nil, "Theme tokens must be allowlisted hexadecimal colors"
		}
		normalizedTokens[key] = strings.ToLower(color)
	}
	normalized := extensions.common(manifest, "theme")
	normalized["permissions"] = []string{}
	normalized["theme"] = map[string]any{"mode": theme["mode"], "tokens": normalizedTokens}
	return normalized, ""
}

func (Extensions) common(manifest map[string]any, kind string) map[string]any {
	return map[string]any{
		"schemaVersion": 1.0,
		"id":            manifest["id"],
		"displayName":   Security{}.Trim(manifest["displayName"].(string)),
		"description":   Security{}.Trim(TextValue{}.Of(manifest["description"])),
		"version":       manifest["version"],
		"kind":          kind,
		"engine":        map[string]any{"caduceus": "26"},
	}
}

func (Extensions) unknownKeys(value map[string]any, allowed []string) bool {
	for key := range value {
		if !(PreferenceChoices{}).Includes(allowed, key) {
			return true
		}
	}
	return false
}

func (Extensions) boundedText(value any, maximum int, required bool) bool {
	text, isText := value.(string)
	return isText && TextValue{}.Length(text) <= maximum && (!required || Security{}.Trim(text) != "") && !strings.ContainsAny(text, "\x00\r\n")
}

func (Extensions) maximumSourceBytes() int { return 128 * 1024 }

func (Extensions) maximumContributions() int { return 12 }
