//go:build responsive

package responsive

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
)

type suite struct {
	restoreFailed bool
	site          *Site
	browser       *Browser
	skipReason    string
	setupError    error
}

type viewport struct {
	width  int64
	height int64
}

var shared suite

var everyWidth = []viewport{
	{320, 800}, {360, 800}, {375, 812}, {414, 896}, {768, 1024},
	{1024, 800}, {1280, 800}, {1440, 900}, {1920, 1080}, {2560, 1440},
}

var bothStates = []AppState{SignInState, SignedInState}

const (
	defaultDist      = "../../apps/web/dist/web"
	safeAreaTop      = 47
	safeAreaBottom   = 34
	safeAreaTopRule  = "env(safe-area-inset-top"
	safeAreaBotRule  = "env(safe-area-inset-bottom"
	viewportFitCover = "viewport-fit=cover"
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	shared = prepareSuite()
	defer shared.close()
	return m.Run()
}

func prepareSuite() suite {
	chromePath, missing := FindChromePath()
	if chromePath == "" {
		return suite{skipReason: missing}
	}
	site, err := StartSite(distFolder())
	if err != nil {
		return suite{setupError: err}
	}
	browser, err := StartBrowser(chromePath)
	if err != nil {
		site.Close()
		return suite{setupError: fmt.Errorf("starting %s: %w", chromePath, err)}
	}
	return suite{site: site, browser: browser}
}

func distFolder() string {
	if configured := os.Getenv("MESAIL_DIST"); configured != "" {
		return configured
	}
	return defaultDist
}

func (s suite) close() {
	if s.browser != nil {
		s.browser.Close()
	}
	if s.site != nil {
		s.site.Close()
	}
}

func requireSuite(t *testing.T) {
	t.Helper()
	if shared.skipReason != "" {
		t.Skip(shared.skipReason)
	}
	if shared.setupError != nil {
		t.Fatal(shared.setupError)
	}
}

func openVisit(t *testing.T, options VisitOptions) *Visit {
	t.Helper()
	options.SignInByForm = shared.restoreFailed && options.State == SignedInState
	visit, err := OpenVisit(shared.browser, shared.site, options)
	if err != nil {
		t.Fatalf("%s: %v", label(options), err)
	}
	t.Cleanup(visit.Close)
	shared.restoreFailed = shared.restoreFailed || visit.RestoreProblem != ""
	return visit
}

func takeSnapshot(t *testing.T, visit *Visit) *Snapshot {
	t.Helper()
	snapshot, err := visit.Snapshot()
	if err != nil {
		t.Fatalf("%s: snapshot: %v", label(visit.Options), err)
	}
	return snapshot
}

func label(options VisitOptions) string {
	pointer := "fine pointer"
	if options.Coarse {
		pointer = "coarse pointer"
	}
	return fmt.Sprintf("%s screen at %dx%d (%s)", options.State, options.Width, options.Height, pointer)
}

func report(t *testing.T, options VisitOptions, findings []string) {
	t.Helper()
	for _, finding := range findings {
		t.Errorf("%s: %s", label(options), finding)
	}
}

func reportConsole(t *testing.T, visit *Visit) {
	t.Helper()
	problems := visit.Problems()
	if len(problems) == 0 {
		return
	}
	report(t, visit.Options, problems)
	if calls := shared.site.UnknownApiCalls(); len(calls) > 0 {
		t.Logf("API calls the fake server does not know: %s", strings.Join(calls, ", "))
	}
}

func requireMedia(t *testing.T, visit *Visit, query string) {
	t.Helper()
	matches, err := visit.MatchesMedia(query)
	if err != nil || !matches {
		t.Fatalf("%s: could not emulate %s (matches=%v, err=%v)", label(visit.Options), query, matches, err)
	}
}

func minimumTarget(options VisitOptions) float64 {
	if options.Coarse || options.Width < narrowLayoutLimit {
		return touchTargetSize
	}
	return pointerTargetSize
}

func checkScreen(t *testing.T, visit *Visit) {
	t.Helper()
	options := visit.Options
	snapshot := takeSnapshot(t, visit)
	report(t, options, CheckHorizontalScroll(snapshot))
	report(t, options, CheckTargetSizes(snapshot, minimumTarget(options)))
	report(t, options, CheckClippedText(snapshot))
	report(t, options, CheckOverlappingText(snapshot))
	if options.State == SignedInState {
		report(t, options, CheckShellLayout(snapshot, options.Width))
	}
	reportConsole(t, visit)
}

func TestSessionRestore(t *testing.T) {
	requireSuite(t)
	options := VisitOptions{Width: 1280, Height: 800, State: SignedInState}
	visit, err := OpenVisit(shared.browser, shared.site, options)
	if err != nil {
		t.Fatalf("%s: %v", label(options), err)
	}
	t.Cleanup(visit.Close)
	shared.restoreFailed = visit.RestoreProblem != ""
	if visit.RestoreProblem != "" {
		t.Errorf("restoring the saved sessions from sessionStorage %s did not reach the shell; the other signed-in checks sign in through the form instead: %s", sessionKeysStorageKey, visit.RestoreProblem)
	}
}

func TestEveryWidth(t *testing.T) {
	requireSuite(t)
	for _, state := range bothStates {
		for _, size := range everyWidth {
			t.Run(fmt.Sprintf("%s/%dpx", state, size.width), func(t *testing.T) {
				checkScreen(t, openVisit(t, VisitOptions{Width: size.width, Height: size.height, State: state}))
			})
		}
	}
}

func TestCoarsePointer(t *testing.T) {
	requireSuite(t)
	for _, state := range bothStates {
		for _, size := range []viewport{{375, 812}, {1024, 768}} {
			t.Run(fmt.Sprintf("%s/%dpx", state, size.width), func(t *testing.T) {
				options := VisitOptions{Width: size.width, Height: size.height, State: state, Coarse: true}
				visit := openVisit(t, options)
				requireMedia(t, visit, "(pointer: coarse)")
				checkScreen(t, visit)
			})
		}
	}
}

func TestReducedMotion(t *testing.T) {
	requireSuite(t)
	reduce := []*emulation.MediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}
	runs := []VisitOptions{
		{Width: 375, Height: 812, State: SignInState, MediaFeatures: reduce},
		{Width: 375, Height: 812, State: SignedInState, MediaFeatures: reduce},
		{Width: 1280, Height: 800, State: SignedInState, MediaFeatures: reduce},
	}
	for _, options := range runs {
		t.Run(fmt.Sprintf("%s/%dpx", options.State, options.Width), func(t *testing.T) {
			visit := openVisit(t, options)
			requireMedia(t, visit, "(prefers-reduced-motion: reduce)")
			report(t, options, CheckReducedMotion(takeSnapshot(t, visit)))
			reportConsole(t, visit)
		})
	}
}

func TestDarkMode(t *testing.T) {
	requireSuite(t)
	dark := []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}}
	for _, state := range bothStates {
		for _, size := range []viewport{{375, 812}, {1440, 900}} {
			t.Run(fmt.Sprintf("%s/%dpx", state, size.width), func(t *testing.T) {
				options := VisitOptions{Width: size.width, Height: size.height, State: state, MediaFeatures: dark}
				visit := openVisit(t, options)
				requireMedia(t, visit, "(prefers-color-scheme: dark)")
				report(t, options, CheckDarkBackground(takeSnapshot(t, visit)))
				reportConsole(t, visit)
			})
		}
	}
}

func TestRightToLeft(t *testing.T) {
	requireSuite(t)
	for _, state := range bothStates {
		for _, size := range []viewport{{375, 812}, {1440, 900}} {
			t.Run(fmt.Sprintf("%s/%dpx", state, size.width), func(t *testing.T) {
				options := VisitOptions{Width: size.width, Height: size.height, State: state}
				visit := openVisit(t, options)
				if err := visit.SetDocumentDirection("rtl"); err != nil {
					t.Fatalf("%s: setting dir=rtl: %v", label(options), err)
				}
				snapshot := takeSnapshot(t, visit)
				report(t, options, prefixed("right-to-left: ", CheckHorizontalScroll(snapshot)))
				report(t, options, prefixed("right-to-left: ", CheckClippedText(snapshot)))
				if state == SignedInState && size.width >= narrowLayoutLimit {
					report(t, options, CheckRailOnRight(snapshot))
				}
				reportConsole(t, visit)
			})
		}
	}
}

func prefixed(prefix string, findings []string) []string {
	for index := range findings {
		findings[index] = prefix + findings[index]
	}
	return findings
}

func TestForcedColors(t *testing.T) {
	requireSuite(t)
	forced := []*emulation.MediaFeature{{Name: "forced-colors", Value: "active"}}
	for _, state := range bothStates {
		t.Run(fmt.Sprintf("%s/1280px", state), func(t *testing.T) {
			options := VisitOptions{Width: 1280, Height: 800, State: state, MediaFeatures: forced}
			visit := openVisit(t, options)
			requireMedia(t, visit, "(forced-colors: active)")
			warnings := focusIndicatorWarnings(t, visit)
			for _, warning := range warnings {
				t.Logf("WARNING %s: forced colors: %s", label(options), warning)
			}
			t.Logf("%s: forced colors: %d focusable elements without an outline or border on focus (warning only, ADR 0004 section 8)", label(options), len(warnings))
			reportConsole(t, visit)
		})
	}
}

func focusIndicatorWarnings(t *testing.T, visit *Visit) []string {
	t.Helper()
	if err := visit.PressTab(); err != nil {
		t.Fatalf("%s: pressing Tab: %v", label(visit.Options), err)
	}
	before := takeSnapshot(t, visit)
	warnings := []string{}
	for _, element := range before.Elements {
		if !element.IsInteractive() || element.IsInert() || !before.IsVisible(element) {
			continue
		}
		if err := visit.Focus(element); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s cannot take focus: %v", before.describeWithName(element), err))
			continue
		}
		focused := findElement(takeSnapshot(t, visit), element)
		if focused == nil || FocusIndicatorWidth(focused) <= 0 {
			warnings = append(warnings, fmt.Sprintf("%s shows no outline or border when focused", before.describeWithName(element)))
		}
	}
	return warnings
}

func findElement(snapshot *Snapshot, wanted *Element) *Element {
	for _, element := range snapshot.Elements {
		if element.BackendNodeID == wanted.BackendNodeID {
			return element
		}
	}
	return nil
}

func TestSafeAreas(t *testing.T) {
	requireSuite(t)
	t.Run("stylesheets", func(t *testing.T) {
		styles := allStylesheets(t, shared.site.DistFolder)
		for _, rule := range []string{safeAreaTopRule, safeAreaBotRule} {
			if !strings.Contains(styles, rule) {
				t.Errorf("no built stylesheet in %s uses %s...)", shared.site.SourceFolder, rule)
			}
		}
	})
	insets := &emulation.SafeAreaInsets{Top: new(int64(safeAreaTop)), Bottom: new(int64(safeAreaBottom)), Left: new(int64(0)), Right: new(int64(0))}
	for _, state := range bothStates {
		t.Run(fmt.Sprintf("%s/375px", state), func(t *testing.T) {
			options := VisitOptions{Width: 375, Height: 812, State: state, Coarse: true, SafeArea: insets, SignInByForm: shared.restoreFailed && state == SignedInState}
			visit, err := OpenVisit(shared.browser, shared.site, options)
			if err != nil && strings.Contains(err.Error(), "setSafeAreaInsetsOverride") {
				t.Skipf("this Chrome cannot emulate safe-area insets (%v); only the stylesheet check ran", err)
			}
			if err != nil {
				t.Fatalf("%s: %v", label(options), err)
			}
			t.Cleanup(visit.Close)
			snapshot := takeSnapshot(t, visit)
			for _, warning := range viewportFitFindings(snapshot) {
				t.Logf("WARNING %s: %s (not a failure: ADR 0004 section 8, \"Safe areas\": Tac pages get no viewport-fit=cover today, so the shell uses TACHYON's --tac-titlebar-height with env(safe-area-inset-*) as the web fallback)", label(options), warning)
			}
			report(t, options, CheckSafeAreaBands(snapshot, safeAreaTop, safeAreaBottom))
			padded := PaddedForTopInset(snapshot, safeAreaTop)
			if padded == nil {
				t.Logf("%s: no visible element has a padding-top of %d px or more, so env(safe-area-inset-top) is not used on this screen or the override did not apply", label(options), safeAreaTop)
			} else {
				t.Logf("%s: the safe-area override applies: %s has padding-top %s", label(options), padded.Describe(), padded.Style["padding-top"])
			}
			reportConsole(t, visit)
		})
	}
}

func viewportFitFindings(snapshot *Snapshot) []string {
	for _, element := range snapshot.Elements {
		if element.Tag != "meta" || element.Attributes["name"] != "viewport" {
			continue
		}
		content := strings.ReplaceAll(element.Attributes["content"], " ", "")
		if strings.Contains(content, viewportFitCover) {
			return nil
		}
		return []string{fmt.Sprintf("<meta name=viewport content=%q> lacks %s, so env(safe-area-inset-*) stays 0 in iOS Safari", element.Attributes["content"], viewportFitCover)}
	}
	return []string{"the page has no <meta name=viewport>"}
}

func allStylesheets(t *testing.T, folder string) string {
	t.Helper()
	var styles strings.Builder
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".css" {
			return err
		}
		content, readErr := os.ReadFile(path)
		styles.Write(content)
		return readErr
	})
	if err != nil {
		t.Fatalf("reading stylesheets in %s: %v", folder, err)
	}
	return styles.String()
}
