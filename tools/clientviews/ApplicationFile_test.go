package main

import (
	"strings"
	"testing"
)

func TestApplicationSettingsWithoutTheSettingAreFine(t *testing.T) {
	problems, err := ApplicationFile{Name: "application.json", Source: []byte(`{"server": {"port": 8104}}`)}.Problems()
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestApplicationSettingsThatKeepTheCheckOnAreFine(t *testing.T) {
	problems, err := ApplicationFile{Name: "application.json", Source: []byte(`{"dependencies": {"check": true}}`)}.Problems()
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestApplicationSettingsThatTurnTheCheckOffAreAProblem(t *testing.T) {
	source := "{\n  \"server\": {\"port\": 8104},\n  \"dependencies\": {\"check\": false}\n}\n"
	problems, err := ApplicationFile{Name: "application.json", Source: []byte(source)}.Problems()
	if err != nil || len(problems) != 1 || problems[0].Line != 3 || !strings.Contains(problems[0].Reason, "dependencies.check") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestATacSettingInApplicationSettingsIsNotTheYonSwitch(t *testing.T) {
	problems, err := ApplicationFile{Name: "application.json", Source: []byte(`{"tac": {"dependencies": {"check": false}}}`)}.Problems()
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestApplicationSettingsThatAreNotJsonAreAnError(t *testing.T) {
	_, err := ApplicationFile{Name: "application.json", Source: []byte(`{"server":`)}.Problems()
	if err == nil || !strings.HasPrefix(err.Error(), "application.json: ") {
		t.Fatalf("got %v", err)
	}
}

func TestApplicationSettingsWithACheckThatIsNotABooleanAreAnError(t *testing.T) {
	_, err := ApplicationFile{Name: "application.json", Source: []byte(`{"dependencies": {"check": "no"}}`)}.Problems()
	if err == nil {
		t.Fatal("want an error")
	}
}
