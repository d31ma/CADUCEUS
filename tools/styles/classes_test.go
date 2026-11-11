package main

import "testing"

func TestClassSourceKeepsClassAttributeValues(t *testing.T) {
	got := classSource([]byte(`<div class="flex gap-2"><span class='sr-only'>x</span><i class=grid></i></div>`))
	if got != "flex gap-2\nsr-only\ngrid\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceIgnoresText(t *testing.T) {
	got := classSource([]byte(`<p class="text-sm">Clear filters, then filter again with a blur and grayscale</p>`))
	if got != "text-sm\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceIgnoresOtherAttributesThatLookLikeClasses(t *testing.T) {
	template := `<w-chip variant="filter" class="w-chip" aria-label="Quick filters" title="flex grid hidden" data-class="block" id="filter" :aria-label="'invert blur'"></w-chip>`
	if got := classSource([]byte(template)); got != "w-chip\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceKeepsLiteralsOfBoundClass(t *testing.T) {
	template := `<li :class="row.active ? 'is-active font-semibold' : &quot;&quot;"></li><p :class='state === "error" ? "text-error" : "text-subtle"'></p>`
	if got := classSource([]byte(template)); got != "is-active font-semibold\nerror\ntext-error\ntext-subtle\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceKeepsStaticPartsOfTemplateLiterals(t *testing.T) {
	template := "<div :class=\"`grid ${row.wide ? 'col-span-2' : ''} gap-2`\"></div>"
	if got := classSource([]byte(template)); got != "grid gap-2\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceIgnoresIdentifiersOfBoundClass(t *testing.T) {
	if got := classSource([]byte(`<section :class="record.cardClass" class="p-4"></section>`)); got != "p-4\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceKeepsQuotedGreaterThanInsideTag(t *testing.T) {
	if got := classSource([]byte(`<p :if="count > 1" class="mt-2">more</p>`)); got != "mt-2\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceIgnoresComments(t *testing.T) {
	if got := classSource([]byte(`<!-- <div class="filter blur"> --><div class="flex"></div>`)); got != "flex\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceReadsUppercaseAttributeNames(t *testing.T) {
	if got := classSource([]byte(`<DIV CLASS="hidden" Data-Class="block"></DIV>`)); got != "hidden\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceSkipsBlankValues(t *testing.T) {
	if got := classSource([]byte(`<div class="  "></div><p class="a
  b"></p>`)); got != "a b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestClassSourceForTemplateWithoutClasses(t *testing.T) {
	if got := classSource([]byte("plain text")); got != "\n" {
		t.Fatalf("got %q", got)
	}
}
