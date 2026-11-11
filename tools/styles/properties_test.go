package main

import (
	"strings"
	"testing"
)

const sampleOutput = `/*! tailwindcss v4.3.5 | MIT License | https://tailwindcss.com */
@layer properties;
@layer utilities {
  .border {
    border-style: var(--tw-border-style);
    border-width: 1px;
  }
}
@property --tw-translate-x {
  syntax: "*";
  inherits: false;
  initial-value: 0;
}
@property --tw-border-style {
  syntax: "*";
  inherits: false;
  initial-value: solid;
}
@layer properties {
  @supports (color: red) {
    *, ::before, ::after, ::backdrop {
      --tw-border-style: solid;
    }
  }
}
`

func TestSplitRegistrationsMovesPropertyBlocksOut(t *testing.T) {
	rules, registrations := splitRegistrations(sampleOutput)
	if strings.Contains(rules, "@property") {
		t.Fatalf("rules still hold @property blocks:\n%s", rules)
	}
	if len(registrations) != 2 {
		t.Fatalf("got %d registrations, want 2", len(registrations))
	}
	want := "@property --tw-border-style {\n  syntax: \"*\";\n  inherits: false;\n  initial-value: solid;\n}"
	if registrations["--tw-border-style"] != want {
		t.Fatalf("got %q, want %q", registrations["--tw-border-style"], want)
	}
}

func TestSplitRegistrationsKeepsUtilitiesAndFallbackLayer(t *testing.T) {
	rules, _ := splitRegistrations(sampleOutput)
	if !strings.Contains(rules, ".border {") || !strings.Contains(rules, "}\n@layer properties {") {
		t.Fatalf("unexpected rules:\n%s", rules)
	}
}

func TestHasRulesIsFalseForHeaderOnlyOutput(t *testing.T) {
	rules, _ := splitRegistrations("/*! tailwindcss v4.3.5 | MIT License | https://tailwindcss.com */\n@layer utilities;\n\n")
	if hasRules(rules) {
		t.Fatalf("header-only output counted as rules: %q", rules)
	}
}

func TestHasRulesIsTrueForUtilities(t *testing.T) {
	rules, _ := splitRegistrations(sampleOutput)
	if !hasRules(rules) {
		t.Fatal("utilities not counted as rules")
	}
}

func TestFoundationBodySortsByName(t *testing.T) {
	body := foundationBody(map[string]string{"--tw-b": "@property --tw-b {}", "--tw-a": "@property --tw-a {}"})
	want := foundationHeader + "\n@property --tw-a {}\n\n@property --tw-b {}"
	if body != want {
		t.Fatalf("got %q, want %q", body, want)
	}
}

func TestFoundationBodyIsEmptyWithoutRegistrations(t *testing.T) {
	if body := foundationBody(map[string]string{}); body != "" {
		t.Fatalf("got %q, want empty", body)
	}
}
