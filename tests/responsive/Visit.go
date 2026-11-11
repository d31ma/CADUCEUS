//go:build responsive

package responsive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/domsnapshot"
	"github.com/chromedp/cdproto/domstorage"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type AppState string

const (
	SignInState   AppState = "sign-in"
	SignedInState AppState = "signed-in"
)

type VisitOptions struct {
	Width         int64
	Height        int64
	Coarse        bool
	State         AppState
	MediaFeatures []*emulation.MediaFeature
	SafeArea      *emulation.SafeAreaInsets
	SignInByForm  bool
}

type Visit struct {
	Options        VisitOptions
	RestoreProblem string
	context        context.Context
	cancel         context.CancelFunc
	messages       []chromedp.ConsoleMessage
	messagesMu     sync.Mutex
	listening      sync.WaitGroup
}

const (
	signedInReadySelector = `nav[aria-label="Domains"]`
	signInReadySelector   = `input[type="password"]`
	visitTimeout          = 120 * time.Second
	settleTime            = 700 * time.Millisecond
	readyTimeout          = 20 * time.Second
	faviconPath           = "/favicon.ico"
	visibilityEventScript = "document.dispatchEvent(new Event('visibilitychange')); 0"
	emailFieldSelector    = `input[autocomplete="username"]`
	passwordFieldSelector = `input[autocomplete="current-password"]`
	signInButtonPath      = `//button[normalize-space(.)="Sign in"]`
)

func OpenVisit(browser *Browser, site *Site, options VisitOptions) (*Visit, error) {
	tabContext, cancelTab := chromedp.NewContext(browser.Context, chromedp.WithNewBrowserContext())
	timedContext, cancelTimeout := context.WithTimeout(tabContext, visitTimeout)
	visit := &Visit{Options: options, context: timedContext, cancel: func() { cancelTimeout(); cancelTab() }}
	if err := chromedp.Do(timedContext); err != nil {
		visit.Close()
		return nil, err
	}
	visit.listenToConsole()
	if err := visit.load(site); err != nil {
		visit.Close()
		return nil, err
	}
	return visit, nil
}

func (visit *Visit) listenToConsole() {
	messages := chromedp.Console(visit.context)
	visit.listening.Go(func() {
		for message, err := range messages {
			if err != nil {
				return
			}
			visit.messagesMu.Lock()
			visit.messages = append(visit.messages, message)
			visit.messagesMu.Unlock()
		}
	})
}

func (visit *Visit) load(site *Site) error {
	if err := visit.step("emulating the device", visit.emulate()); err != nil {
		return err
	}
	if visit.Options.SignInByForm {
		return visit.signInThroughForm(site)
	}
	if err := visit.storeSessionKeysWhenSignedIn(site); err != nil {
		return err
	}
	if err := visit.step("opening the app", chromedp.Navigate(site.Url("/"))); err != nil {
		return err
	}
	err := visit.waitUntilReady(site)
	if err == nil || visit.Options.State != SignedInState {
		return err
	}
	visit.RestoreProblem = err.Error()
	if visit.renderedAfterVisibilityEvent(site) {
		visit.RestoreProblem += "; the restored state was there: one synthetic visibilitychange event (which makes Tac publish VisibilityChanged and render) showed the shell at once, so nothing re-rendered the view after the @OnEvent @InBackground Restore task changed session.Step"
		return nil
	}
	visit.RestoreProblem += "; a synthetic visibilitychange event did not show the shell either"
	return visit.signInThroughForm(site)
}

func (visit *Visit) renderedAfterVisibilityEvent(site *Site) bool {
	if err := visit.step("sending visibilitychange", chromedp.Evaluate[chromedp.Void](visibilityEventScript)); err != nil {
		return false
	}
	return visit.waitUntilReady(site) == nil
}

func (visit *Visit) storeSessionKeysWhenSignedIn(site *Site) error {
	if visit.Options.State != SignedInState {
		return nil
	}
	if err := visit.step("opening the blank page", chromedp.Navigate(site.Url(blankPagePath))); err != nil {
		return err
	}
	return visit.step("storing the session keys", visit.storeSessionKeys(site))
}

func (visit *Visit) step(name string, action chromedp.Action[chromedp.Void]) error {
	stepContext, cancelStep := context.WithTimeout(visit.context, readyTimeout)
	defer cancelStep()
	if err := chromedp.Do(stepContext, action); err != nil {
		return fmt.Errorf("%s for the %s screen: %w", name, visit.Options.State, err)
	}
	return nil
}

type formStep struct {
	name   string
	action chromedp.Action[chromedp.Void]
}

func (visit *Visit) signInThroughForm(site *Site) error {
	steps := []formStep{
		{"opening the blank page", chromedp.Navigate(site.Url(blankPagePath))},
		{"forgetting the session keys", visit.forgetSessionKeys(site)},
		{"opening the sign-in form", chromedp.Navigate(site.Url("/"))},
		{"typing the email into " + emailFieldSelector, chromedp.SendKeys(chromedp.CSS(emailFieldSelector), "ana@example.com")},
		{"typing the password into " + passwordFieldSelector, chromedp.SendKeys(chromedp.CSS(passwordFieldSelector), rightPassword)},
		{"pressing " + signInButtonPath, chromedp.Click(chromedp.Search(signInButtonPath))},
	}
	for _, step := range steps {
		if err := visit.step(step.name, step.action); err != nil {
			return fmt.Errorf("signing in through the form: %w", err)
		}
	}
	return visit.waitUntilReady(site)
}

func (visit *Visit) waitUntilReady(site *Site) error {
	waitContext, cancelWait := context.WithTimeout(visit.context, readyTimeout)
	defer cancelWait()
	err := chromedp.Do(waitContext, chromedp.WaitReady(chromedp.CSS(visit.readySelector())), chromedp.Sleep(settleTime))
	if err == nil {
		return nil
	}
	return fmt.Errorf("the %s screen did not appear within %s (no %s); the page shows: %s; every console and log entry: [%s]; API calls so far: %s",
		visit.Options.State, readyTimeout, visit.readySelector(), visit.visibleText(), strings.Join(visit.AllMessages(), " | "), strings.Join(site.ApiCalls(), ", "))
}

func (visit *Visit) visibleText() string {
	snapshot, err := visit.Snapshot()
	if err != nil {
		return "(no snapshot: " + err.Error() + ")"
	}
	parts := []string{}
	for _, element := range snapshot.Elements {
		if element.DirectText != "" && snapshot.IsVisible(element) && len(parts) < 12 {
			parts = append(parts, shorten(element.DirectText))
		}
	}
	return strings.Join(parts, " / ")
}

func (visit *Visit) readySelector() string {
	if visit.Options.State == SignedInState {
		return signedInReadySelector
	}
	return signInReadySelector
}

func (visit *Visit) emulate() chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
		if err := visit.emulateViewport(ctx, target); err != nil {
			return err
		}
		if _, err := cdp.Call(ctx, target, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Features: visit.Options.MediaFeatures}); err != nil {
			return err
		}
		if visit.Options.SafeArea == nil {
			return nil
		}
		_, err := cdp.Call(ctx, target, emulation.SetSafeAreaInsetsOverride, emulation.SetSafeAreaInsetsOverrideParams{Insets: visit.Options.SafeArea})
		return err
	})
}

func (visit *Visit) emulateViewport(ctx context.Context, target *chromedp.Target) error {
	metrics := emulation.SetDeviceMetricsOverrideParams{Width: visit.Options.Width, Height: visit.Options.Height, DeviceScaleFactor: 1, Mobile: visit.Options.Coarse}
	if _, err := cdp.Call(ctx, target, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		return err
	}
	touch := emulation.SetTouchEmulationEnabledParams{Enabled: visit.Options.Coarse}
	if visit.Options.Coarse {
		touch.MaxTouchPoints = 5
	}
	_, err := cdp.Call(ctx, target, emulation.SetTouchEmulationEnabled, touch)
	return err
}

func (visit *Visit) storeSessionKeys(site *Site) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
		_, err := cdp.Call(ctx, target, domstorage.SetDOMStorageItem, domstorage.SetDOMStorageItemParams{
			StorageID: &domstorage.StorageID{SecurityOrigin: site.Origin(), IsLocalStorage: false},
			Key:       site.Profile.SessionStorageKey,
			Value:     site.Profile.StoredSessionText(),
		})
		return err
	})
}

func (visit *Visit) forgetSessionKeys(site *Site) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, target *chromedp.Target) error {
		_, err := cdp.Call(ctx, target, domstorage.RemoveDOMStorageItem, domstorage.RemoveDOMStorageItemParams{
			StorageID: &domstorage.StorageID{SecurityOrigin: site.Origin(), IsLocalStorage: false},
			Key:       site.Profile.SessionStorageKey,
		})
		return err
	})
}

func (visit *Visit) Snapshot() (*Snapshot, error) {
	capture, err := chromedp.Call(visit.context, domsnapshot.CaptureSnapshot, domsnapshot.CaptureSnapshotParams{
		ComputedStyles:  SnapshotStyles,
		IncludeDOMRects: new(true),
	})
	if err != nil {
		return nil, err
	}
	tree, err := chromedp.Call(visit.context, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		return nil, err
	}
	snapshot, err := NewSnapshot(capture, tree.Nodes)
	if err != nil {
		return nil, err
	}
	return snapshot, visit.addLayoutMetrics(snapshot)
}

func (visit *Visit) addLayoutMetrics(snapshot *Snapshot) error {
	metrics, err := chromedp.Call(visit.context, page.GetLayoutMetrics, cdp.Empty{})
	if err != nil {
		return err
	}
	snapshot.ViewportWidth = float64(metrics.CSSLayoutViewport.ClientWidth)
	snapshot.ViewportHeight = float64(metrics.CSSLayoutViewport.ClientHeight)
	snapshot.ContentWidth = metrics.CSSContentSize.Width
	snapshot.ContentHeight = metrics.CSSContentSize.Height
	snapshot.ContentLeft = metrics.CSSContentSize.X
	return nil
}

func (visit *Visit) MatchesMedia(query string) (bool, error) {
	encoded, _ := json.Marshal(query)
	return chromedp.Run(visit.context, chromedp.Evaluate[bool]("matchMedia("+string(encoded)+").matches"))
}

func (visit *Visit) SetDocumentDirection(direction string) error {
	document, err := chromedp.Call(visit.context, dom.GetDocument, dom.GetDocumentParams{})
	if err != nil {
		return err
	}
	html, err := chromedp.Call(visit.context, dom.QuerySelector, dom.QuerySelectorParams{NodeID: document.Root.NodeID, Selector: "html"})
	if err != nil {
		return err
	}
	if _, err := chromedp.Call(visit.context, dom.SetAttributeValue, dom.SetAttributeValueParams{NodeID: html.NodeID, Name: "dir", Value: direction}); err != nil {
		return err
	}
	return chromedp.Do(visit.context, chromedp.Sleep(settleTime))
}

func (visit *Visit) PressTab() error {
	for _, eventType := range []input.DispatchKeyEventType{input.DispatchKeyEventTypeKeyDown, input.DispatchKeyEventTypeKeyUp} {
		_, err := chromedp.Call(visit.context, input.DispatchKeyEvent, input.DispatchKeyEventParams{
			Type: eventType, Key: "Tab", Code: "Tab", WindowsVirtualKeyCode: 9, NativeVirtualKeyCode: 9,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (visit *Visit) Focus(element *Element) error {
	_, err := chromedp.Call(visit.context, dom.Focus, dom.FocusParams{BackendNodeID: element.BackendNodeID})
	return err
}

func (visit *Visit) AllMessages() []string {
	visit.messagesMu.Lock()
	defer visit.messagesMu.Unlock()
	all := []string{}
	for _, message := range visit.messages {
		all = append(all, describeMessage(message))
	}
	return all
}

func (visit *Visit) Problems() []string {
	visit.messagesMu.Lock()
	defer visit.messagesMu.Unlock()
	problems := []string{}
	for _, message := range visit.messages {
		if isProblem(message) {
			problems = append(problems, describeMessage(message))
		}
	}
	return problems
}

func isProblem(message chromedp.ConsoleMessage) bool {
	if strings.Contains(message.Text, "Content Security Policy") {
		return true
	}
	if strings.HasSuffix(message.URL, faviconPath) {
		return false
	}
	return message.Type == chromedp.ConsoleError || message.Type == chromedp.ConsoleException
}

func describeMessage(message chromedp.ConsoleMessage) string {
	kind := "console " + string(message.Type)
	if strings.Contains(message.Text, "Content Security Policy") {
		kind = "CSP violation"
	}
	place := ""
	if message.URL != "" {
		place = fmt.Sprintf(" at %s:%d", message.URL, message.Line+1)
	}
	return fmt.Sprintf("%s%s: %s", kind, place, message.Text)
}

func (visit *Visit) Close() {
	visit.cancel()
	visit.listening.Wait()
}
