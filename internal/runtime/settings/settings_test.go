package settings

import (
	"reflect"
	"testing"
)

// The expectations are ServerEnvironment.dotenv's and .value's answers.
func TestDotenvMatchesServerEnvironment(t *testing.T) {
	text, skipped := Dotenv(map[string]string{
		"FYLO_ROOT": "/data/caduceus", "MAIL_TLS_KEY": "a\nb", "SMTP_ADAPTER": "direct", "EMPTY": "",
		"NODE_ENV": "production", "OTHER": "x", "INBOUND_WEBHOOK_SECRET": "s=1 #2", "DKIM_SELECTOR": "",
	})
	want := "# Written by scripts/entry.mjs from the process environment.\nYON_FYLO_ROOT=/data/caduceus\nYON_INBOUND_WEBHOOK_SECRET=s=1 #2\nYON_NODE_ENV=production\nYON_SMTP_ADAPTER=direct\n"
	if text != want || !reflect.DeepEqual(skipped, []string{"MAIL_TLS_KEY"}) {
		t.Fatalf("text = %q skipped = %v", text, skipped)
	}
}

func TestLookupFallsBackToYonApplication(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"smtp":{"inbound":{"port":2525}},"mail":{"auth":{"verify":false}},"fylo":{"root":{"x":1}}}}`)
	if got := Value("SMTP_INBOUND_PORT"); got != "2525" {
		t.Fatalf("port = %q", got)
	}
	if got := Value("MAIL_AUTH_VERIFY"); got != "false" {
		t.Fatalf("verify = %q", got)
	}
	if _, ok := Lookup("FYLO_ROOT"); ok {
		t.Fatal("object setting answered")
	}
	t.Setenv("SMTP_INBOUND_PORT", "")
	if got := Or("SMTP_INBOUND_PORT", "25"); got != "" {
		t.Fatalf("empty variable not kept: %q", got)
	}
}

func TestNumbers(t *testing.T) {
	cases := map[string]float64{"": 30, "abc": 30, "-5": 30, "0": 30, "15000": 15000, " 1e3 ": 1000, "0x10": 16, "Infinity": 30}
	for input, want := range cases {
		if got := PositiveNumber(input, 30); got != want {
			t.Fatalf("PositiveNumber(%q) = %v, want %v", input, got, want)
		}
	}
	if got := NonNegativeNumber("0", 5); got != 0 {
		t.Fatalf("NonNegativeNumber(0) = %v", got)
	}
	if got := NonNegativeNumber("", 5); got != 0 {
		t.Fatalf("NonNegativeNumber('') = %v (Number('') is 0)", got)
	}
}

func TestEnvNumbersTreatUnsetAsNaN(t *testing.T) {
	if got := EnvNonNegative("CADUCEUS_TEST_UNSET_NUMBER", 300000); got != 300000 {
		t.Fatalf("unset = %v", got)
	}
	t.Setenv("CADUCEUS_TEST_SET_NUMBER", "0")
	if got := EnvNonNegative("CADUCEUS_TEST_SET_NUMBER", 300000); got != 0 {
		t.Fatalf("zero = %v", got)
	}
	if got := EnvPositive("CADUCEUS_TEST_SET_NUMBER", 30000); got != 30000 {
		t.Fatalf("positive zero = %v", got)
	}
}
