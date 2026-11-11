package webpush

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultHostsAcceptTheKnownPushServices(t *testing.T) {
	policy := EndpointPolicy{Hosts: EndpointPolicy{}.DefaultHosts()}
	accepted := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://web.push.apple.com/QGx",
		"https://wns2-par02p.notify.windows.com/w/?token=abc",
		"https://FCM.googleapis.com:443/fcm/send/abc",
	}
	for _, endpoint := range accepted {
		err := policy.Check(endpoint)
		if err != nil {
			t.Fatalf("%s: %v", endpoint, err)
		}
	}
}

func TestPolicyRefusesEverythingElse(t *testing.T) {
	policy := EndpointPolicy{Hosts: []string{"push.apple.com", ""}}
	cases := map[string]string{
		"http://web.push.apple.com/x":          "https",
		"https://user:secret@push.apple.com/x": "user name",
		"https://push.apple.com:8443/x":        "port 443",
		"https://evilpush.apple.com/x":         "known push service",
		"https://push.apple.com.evil.com/x":    "known push service",
		"https://127.0.0.1/x":                  "known push service",
		"https://169.254.169.254/latest":       "known push service",
		"https:///x":                           "known push service",
		"https://push.apple.com./x":            "known push service",
		"https://exa mple.com/":                "not a valid URL",
		"push.apple.com/x":                     "https",
	}
	for endpoint, reason := range cases {
		t.Run(endpoint, func(t *testing.T) {
			err := policy.Check(endpoint)
			refused := RefusedEndpoint{}
			if !errors.As(err, &refused) {
				t.Fatalf("error %v", err)
			}
			if !strings.Contains(err.Error(), reason) {
				t.Fatalf("message %q does not contain %q", err.Error(), reason)
			}
		})
	}
}

func TestEmptyPolicyRefusesAllHosts(t *testing.T) {
	err := EndpointPolicy{}.Check("https://fcm.googleapis.com/x")
	if err == nil {
		t.Fatal("expected a refusal")
	}
}
