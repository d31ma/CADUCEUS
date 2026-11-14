package main

import (
	"strings"
	"testing"
)

func environmentProblems(source string) []Problem {
	return EnvironmentFile{Name: ".env", Source: source}.Problems()
}

func TestAnEnvironmentFileWithoutTheSwitchIsFine(t *testing.T) {
	if problems := environmentProblems("YON_DATASOURCE_DRIVER=fylo\nYON_SERVICE_CLIENT_SECRET=\n"); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestAnEnvironmentFileThatTurnsTheTacCheckOffIsAProblem(t *testing.T) {
	problems := environmentProblems("YON_DATASOURCE_DRIVER=fylo\nTAC_DEPENDENCIES_CHECK=false\n")
	if len(problems) != 1 || problems[0].Line != 2 || !strings.Contains(problems[0].Reason, "TAC_DEPENDENCIES_CHECK=false") {
		t.Fatalf("got %v", problems)
	}
}

func TestAnExportedQuotedZeroTurnsTheCheckOff(t *testing.T) {
	if problems := environmentProblems(`export TAC_DEPENDENCIES_CHECK="0"`); len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestOffInCapitalsWithACommentTurnsTheCheckOff(t *testing.T) {
	if problems := environmentProblems("TAC_DEPENDENCIES_CHECK = OFF # while we migrate"); len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestTheYonSwitchTurnedOffIsAProblem(t *testing.T) {
	if problems := environmentProblems("YON_DEPENDENCIES_CHECK=no"); len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestTheSwitchTurnedOnIsFine(t *testing.T) {
	if problems := environmentProblems("TAC_DEPENDENCIES_CHECK=true"); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestACommentedOutSwitchIsFine(t *testing.T) {
	if problems := environmentProblems("# TAC_DEPENDENCIES_CHECK=false"); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestAnotherSettingTurnedOffIsFine(t *testing.T) {
	if problems := environmentProblems("TAC_NAMES_CHECK=false"); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}
