package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func cachedTailwind(t *testing.T) tailwind {
	t.Helper()
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	directory, err := cacheDirectory()
	if err != nil {
		t.Skip(err)
	}
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("Tailwind binary not cached at %s; run the tool once to download it", path)
	}
	if err := verify(path, assetChecksum(name)); err != nil {
		t.Fatal(err)
	}
	return tailwind{path: path}
}

func TestTailwindBuildsLayoutUtilitiesWithDuVayBreakpoints(t *testing.T) {
	binary := cachedTailwind(t)
	template := `<div class="grid grid-cols-1 sm:flex md:grid-cols-[4.25rem_17rem_1fr] lg:grid-cols-[4.25rem_17rem_1fr_25rem] xl:gap-4 2xl:gap-6 gap-0 min-h-dvh overflow-hidden px-4 pt-[env(safe-area-inset-top)]"></div>`
	css, err := binary.compile([]byte(template))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"@media (width >= 600px) {\n    .sm\\:flex {",
		"@media (width >= 960px) {\n    .md\\:grid-cols-\\[4\\.25rem_17rem_1fr\\] {\n      grid-template-columns: 4.25rem 17rem 1fr;",
		"@media (width >= 1280px) {\n    .lg\\:grid-cols-\\[4\\.25rem_17rem_1fr_25rem\\] {",
		"@media (width >= 1920px) {\n    .xl\\:gap-4 {",
		"@media (width >= 2560px) {\n    .\\32 xl\\:gap-6 {",
		"padding-inline: calc(var(--spacing, 0.25rem) * 4);",
		"padding-top: env(safe-area-inset-top);",
		"min-height: 100dvh;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("output lacks %q:\n%s", want, css)
		}
	}
}

func TestTailwindDropsDefaultBreakpointWidths(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<div class="sm:flex md:flex lg:flex xl:flex 2xl:flex"></div>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"40rem", "48rem", "64rem", "80rem", "96rem"} {
		if strings.Contains(css, absent) {
			t.Errorf("output holds default breakpoint %q:\n%s", absent, css)
		}
	}
}

func TestTailwindBuildsContainerQueries(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<section class="@container"><div class="@md:flex @min-[600px]:grid"></div></section>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"container-type: inline-size;", "@container (width >= 28rem)", "@container (width >= 600px)"} {
		if !strings.Contains(css, want) {
			t.Errorf("output lacks %q:\n%s", want, css)
		}
	}
}

func TestTailwindOmitsPaletteAndUnmappedTypeUtilities(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<p class="bg-red-500 text-3xl font-extrabold leading-tight shadow-xl drop-shadow-md text-shadow-sm font-serif flex">text</p>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{".bg-red-500", ".text-3xl", ".font-extrabold", ".leading-tight", ".shadow-xl", ".drop-shadow-md", ".text-shadow-sm", ".font-serif", "oklch", "rgb("} {
		if strings.Contains(css, absent) {
			t.Errorf("output holds %q:\n%s", absent, css)
		}
	}
	if !strings.Contains(css, ".flex {") {
		t.Errorf("output lacks .flex:\n%s", css)
	}
}

func TestTailwindMapsColorTypeRadiusAndShadowToDuVayTokens(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<p class="bg-surface text-subtle border-divider text-sm font-semibold font-mono tracking-wide leading-snug rounded-lg shadow-md">text</p>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"background-color: var(--w-surface);",
		"color: var(--w-text-subtle);",
		"border-color: var(--w-divider);",
		"font-size: var(--w-font-base);",
		"line-height: var(--tw-leading, calc(1.25 / 0.875));",
		"font-weight: var(--w-font-weight-semibold);",
		"font-family: var(--w-font-family-mono);",
		"letter-spacing: var(--w-letter-spacing-wide);",
		"line-height: var(--w-line-height-snug);",
		"border-radius: var(--w-radius-lg);",
		"--tw-shadow: var(--w-shadow-md);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("output lacks %q:\n%s", want, css)
		}
	}
	rules, _ := splitRegistrations(css)
	if colors := hardCodedColors(rules); len(colors) != 0 {
		t.Errorf("guard rejects mapped tokens: %v", colors)
	}
}

func TestTailwindBuildsMockupUtilitiesAndVariants(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<div class="@container/page safe-t safe-l touch-target drag-region short:hidden @max-2xl/page:block bg-on-rail/20"></div>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"padding-top: max(env(safe-area-inset-top, 0px), var(--tac-titlebar-height, 0px));",
		"padding-left: max(var(--ms-edge, 0px), env(safe-area-inset-left, 0px), var(--tac-titlebar-leading-inset, 0px));",
		"min-height: var(--w-touch-min, 2.75rem);",
		"app-region: drag;",
		"@media (max-height: 520px)",
		"@container page (width < 42rem)",
		"color-mix(in oklab, var(--ms-on-rail) 20%, transparent)",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("output lacks %q:\n%s", want, css)
		}
	}
	rules, _ := splitRegistrations(css)
	if colors := hardCodedColors(rules); len(colors) != 0 {
		t.Errorf("guard rejects token opacity: %v", colors)
	}
}

func TestTailwindIgnoresWordsOutsideClassAttributes(t *testing.T) {
	binary := cachedTailwind(t)
	template := `<w-chip variant="filter" aria-label="Quick filters" class="w-chip">Clear filters</w-chip><p title="blur grayscale invert">filter</p>`
	css, err := binary.compile([]byte(template))
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"filter:", "@property", "--tw-blur", ".invert", ".blur"} {
		if strings.Contains(css, absent) {
			t.Errorf("output holds %q:\n%s", absent, css)
		}
	}
}

func TestTailwindBuildsLiteralsOfBoundClass(t *testing.T) {
	binary := cachedTailwind(t)
	css, err := binary.compile([]byte(`<li :class="row.open ? 'grid' : 'hidden'">row</li>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".grid {", ".hidden {"} {
		if !strings.Contains(css, want) {
			t.Errorf("output lacks %q:\n%s", want, css)
		}
	}
	if strings.Contains(css, ".row") || strings.Contains(css, ".open") {
		t.Errorf("output holds identifiers:\n%s", css)
	}
}

func TestTailwindOutputIsDeterministic(t *testing.T) {
	binary := cachedTailwind(t)
	template := []byte(`<div class="flex gap-2 tablet:hidden translate-x-2 border"></div>`)
	first, err := binary.compile(template)
	if err != nil {
		t.Fatal(err)
	}
	second, err := binary.compile(template)
	if err != nil || first != second {
		t.Fatalf("outputs differ or failed: %v", err)
	}
}
