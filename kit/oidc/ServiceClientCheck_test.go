package oidc

import "testing"

func serviceCheck() ServiceClientCheck {
	return ServiceClientCheck{}.FromJson(`{"yon":{"service":{"client":{"secret":"current-secret","previous":"previous-secret"}}}}`)
}

func TestServiceClientCheckAcceptsTheCurrentSecret(t *testing.T) {
	if !serviceCheck().Accepts("mesail-service", "current-secret") {
		t.Fatal("refused the current secret")
	}
}

func TestServiceClientCheckAcceptsThePreviousSecret(t *testing.T) {
	if !serviceCheck().Accepts("mesail-service", "previous-secret") {
		t.Fatal("refused the previous secret")
	}
}

func TestServiceClientCheckRefusesAWrongSecret(t *testing.T) {
	if serviceCheck().Accepts("mesail-service", "wrong-secret") {
		t.Fatal("accepted a wrong secret")
	}
}

func TestServiceClientCheckRefusesAnotherClient(t *testing.T) {
	if serviceCheck().Accepts("mesail-identity", "current-secret") {
		t.Fatal("accepted another client")
	}
}

func TestServiceClientCheckRefusesAnEmptySecretWhenNoPreviousIsSet(t *testing.T) {
	check := ServiceClientCheck{}.FromJson(`{"yon":{"service":{"client":{"secret":"current-secret"}}}}`)
	if check.Accepts("mesail-service", "") {
		t.Fatal("accepted an empty secret")
	}
}

func TestServiceClientCheckReadsTheEnvironment(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"service":{"client":{"secret":"env-secret"}}}}`)
	if !(ServiceClientCheck{}).Configured().Accepts("mesail-service", "env-secret") {
		t.Fatal("did not read the environment")
	}
}
