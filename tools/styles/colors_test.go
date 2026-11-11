package main

import "testing"

func colorsIn(declaration string) []string {
	return hardCodedColors("@layer utilities {\n  .x {\n    " + declaration + "\n  }\n}")
}

func TestHardCodedColorsAcceptsLayoutDeclarations(t *testing.T) {
	if found := colorsIn("grid-template-columns: 4.25rem 17rem 1fr;"); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsAcceptsDuVayToken(t *testing.T) {
	if found := colorsIn("background-color: var(--w-surface);"); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsAcceptsCurrentColor(t *testing.T) {
	if found := colorsIn("border-color: currentcolor;"); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsRejectsHex(t *testing.T) {
	if found := colorsIn("background-color: #fff;"); len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsRejectsColorFunctionInsideFallback(t *testing.T) {
	if found := colorsIn("color: var(--color-red-500, oklch(63.7% 0.237 25.331));"); len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsRejectsNamedColor(t *testing.T) {
	if found := colorsIn("color: red;"); len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsIgnoresEscapedHashInSelector(t *testing.T) {
	found := hardCodedColors("@layer utilities {\n  .w-\\[\\#abc\\] {\n    width: 1px;\n  }\n}")
	if len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsIgnoresPropertiesFallbackLayer(t *testing.T) {
	rules := "@layer utilities {\n  .ring {\n    width: 1px;\n  }\n}\n@layer properties {\n  @supports (color: red) {\n    * {\n      --tw-ring-offset-color: #fff;\n    }\n  }\n}"
	if found := hardCodedColors(rules); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsAcceptsOpacityMixOfDuVayToken(t *testing.T) {
	if found := colorsIn("background-color: color-mix(in oklab, var(--ms-on-rail) 20%, transparent);"); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsRejectsOpacityMixOfHex(t *testing.T) {
	if found := colorsIn("background-color: color-mix(in oklab, #ff0000 20%, transparent);"); len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsRejectsMixOfNamedColor(t *testing.T) {
	if found := colorsIn("color: color-mix(in srgb, red 50%, var(--w-text));"); len(found) != 1 {
		t.Fatalf("got %v", found)
	}
}

func TestHardCodedColorsAcceptsSystemColor(t *testing.T) {
	if found := colorsIn("outline-color: Highlight;"); len(found) != 0 {
		t.Fatalf("got %v", found)
	}
}
