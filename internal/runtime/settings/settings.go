// Package settings reads CADUCEUS deployment settings by their environment
// names and writes the `.env` that hands them to Tachyon's handler processes.
//
// It ports tests/support/legacy-server/shared/runtime/ServerEnvironment.js.
// HandlerSettings is the one list of what reaches the handlers. The Go
// server's reader (server/shared/runtime/Settings.go) answers any name from
// the environment or YON_APPLICATION_JSON and keeps no list of its own;
// server_settings_test.go checks that every name it reads is listed here and
// that tests/support/TestServer.mjs hands the suites the same list.
package settings

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// GeneratedMarker opens a `.env` the entry point wrote, so it never replaces
// one a person wrote. It is unchanged from the JavaScript entry point so a
// file that version wrote is still recognized.
const GeneratedMarker = "# Written by scripts/entry.mjs from the process environment."

// HandlerSettings are the settings a handler process reads; the entry point
// hands them to Tachyon.
var HandlerSettings = []string{
	"ATTACHMENT_ROOT",
	"CADUCEUS_API_URL",
	"CADUCEUS_ENABLE_TEST_ROUTES",
	"CADUCEUS_PUBLIC_URL",
	"CADUCEUS_TEST_REAL_TRANSLATION",
	"DKIM_DOMAIN",
	"DKIM_PRIVATE_KEY",
	"DKIM_PRIVATE_KEY_FILE",
	"DKIM_SELECTOR",
	"FYLO_BINARY",
	"FYLO_BRIDGE_ADDRESS",
	"FYLO_BRIDGE_SECRET",
	"FYLO_EXEC_PATH",
	"FYLO_INCOMING_ROOT",
	"FYLO_POSIX_ACCESS",
	"FYLO_ROOT",
	"INBOUND_WEBHOOK_SECRET",
	"MAIL_AUTH_VERIFY",
	"MAIL_HOSTNAME",
	"MAIL_TLS_CERT",
	"MAIL_TLS_CERT_FILE",
	"MAIL_TLS_KEY",
	"MAIL_TLS_KEY_FILE",
	"NODE_ENV",
	"SESAME_ISSUER",
	"SMTP_ADAPTER",
	"SMTP_FROM",
	"SMTP_INBOUND_HOST",
	"SMTP_INBOUND_PORT",
	"SMTP_MAX_SIZE",
	"SMTP_SUBMISSION_HOST",
	"SMTP_TLS_POLICY",
	"SPAM_FILTER_DISABLED",
	"SPAM_SCORE_THRESHOLD",
	"STREAM_MAX_MS",
	"STREAM_POLL_MS",
	"STREAM_REPLAY_LIMIT",
	"TRACKING_SECRET",
	"TRANSLATION_ASSET_ROOT",
	"VAPID_PRIVATE_KEY",
	"VAPID_PUBLIC_KEY",
	"VAPID_SUBJECT",
	"WEB_PUSH_DISABLED",
}

// Lookup answers one setting: the environment variable when it is set (even
// to ""), otherwise the matching `yon.*` value of YON_APPLICATION_JSON, as
// ServerEnvironment.value did.
func Lookup(name string) (string, bool) {
	if value, ok := os.LookupEnv(name); ok {
		return value, true
	}
	return configured(name)
}

// Value answers Lookup's value, or "" when the setting is absent.
func Value(name string) string {
	value, _ := Lookup(name)
	return value
}

// Or answers the setting, or fallback when it is absent. Like the
// JavaScript `??`, a setting that is present but empty is kept.
func Or(name, fallback string) string {
	if value, ok := Lookup(name); ok {
		return value
	}
	return fallback
}

var (
	decimalNumber = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	hexNumber     = regexp.MustCompile(`^0[xX][0-9a-fA-F]+$`)
)

// Number ports JavaScript `Number(value)` for strings: it answers ok=false
// where Number answered NaN.
func Number(value string) (float64, bool) {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return 0, true
	case trimmed == "Infinity" || trimmed == "+Infinity":
		return math.Inf(1), true
	case trimmed == "-Infinity":
		return math.Inf(-1), true
	case decimalNumber.MatchString(trimmed):
		parsed, err := strconv.ParseFloat(trimmed, 64)
		return parsed, err == nil || math.IsInf(parsed, 0)
	case hexNumber.MatchString(trimmed):
		parsed, err := strconv.ParseUint(trimmed[2:], 16, 64)
		return float64(parsed), err == nil
	}
	return 0, false
}

// PositiveNumber ports entry.mjs positiveNumber: a finite number above 0,
// else fallback.
func PositiveNumber(value string, fallback float64) float64 {
	parsed, ok := Number(value)
	if ok && parsed > 0 && !math.IsInf(parsed, 0) {
		return parsed
	}
	return fallback
}

// NonNegativeNumber ports entry.mjs nonNegativeNumber: a finite number of at
// least 0, else fallback.
func NonNegativeNumber(value string, fallback float64) float64 {
	parsed, ok := Number(value)
	if ok && parsed >= 0 && !math.IsInf(parsed, 0) {
		return parsed
	}
	return fallback
}

var newline = regexp.MustCompile(`[\r\n]`)

// Dotenv answers the project `.env` that gives Tachyon's handler processes
// the settings in environment. Only single-line values can be written; a
// multi-line value, such as a PEM key, is skipped and named in skipped.
func Dotenv(environment map[string]string) (text string, skipped []string) {
	lines := []string{GeneratedMarker}
	for _, name := range HandlerSettings {
		value := environment[name]
		if value == "" {
			continue
		}
		if newline.MatchString(value) {
			skipped = append(skipped, name)
			continue
		}
		lines = append(lines, "YON_"+name+"="+value)
	}
	return strings.Join(lines, "\n") + "\n", skipped
}

func configured(name string) (string, bool) {
	text := os.Getenv("YON_APPLICATION_JSON")
	if text == "" {
		return "", false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return "", false
	}
	var node any = parsed["yon"]
	if _, ok := node.(map[string]any); !ok {
		return "", false
	}
	for _, key := range strings.Split(strings.ToLower(name), "_") {
		object, ok := node.(map[string]any)
		if !ok {
			return "", false
		}
		node = object[key]
	}
	switch typed := node.(type) {
	case nil, map[string]any, []any:
		return "", false
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	default:
		return "", false
	}
}

// EnvPositive reads an environment variable as entry.mjs positiveNumber did:
// unset (Number(undefined) is NaN) or not a finite positive number answers
// fallback.
func EnvPositive(name string, fallback float64) float64 {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return PositiveNumber(value, fallback)
}

// EnvNonNegative reads an environment variable as entry.mjs
// nonNegativeNumber did; an empty value is Number(”) = 0.
func EnvNonNegative(name string, fallback float64) float64 {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return NonNegativeNumber(value, fallback)
}
