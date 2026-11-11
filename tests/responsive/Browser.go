//go:build responsive

package responsive

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

const (
	chromiumPrefix      = "chromium-"
	headlessShellPrefix = "chromium_headless_shell-"
)

type Browser struct {
	Context         context.Context
	cancelBrowser   context.CancelFunc
	cancelAllocator context.CancelFunc
}

func FindChromePath() (string, string) {
	override := os.Getenv("MESAIL_CHROME")
	if override != "" {
		return checkedChromePath(override)
	}
	for _, folder := range playwrightFolders() {
		found := newestPlaywrightChrome(folder)
		if found != "" {
			return found, ""
		}
	}
	return "", "no Chromium found: set MESAIL_CHROME, or install Playwright's chromium (or chromium-headless-shell) into $PLAYWRIGHT_BROWSERS_PATH or ~/Library/Caches/ms-playwright"
}

func checkedChromePath(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", "MESAIL_CHROME=" + path + " is not a file"
	}
	return path, ""
}

func playwrightFolders() []string {
	home, _ := os.UserHomeDir()
	folders := []string{}
	if configured := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); configured != "" && configured != "0" {
		folders = append(folders, configured)
	}
	return append(folders,
		filepath.Join(home, "Library", "Caches", "ms-playwright"),
		filepath.Join(home, ".cache", "ms-playwright"),
		filepath.Join(home, "AppData", "Local", "ms-playwright"),
	)
}

func newestPlaywrightChrome(folder string) string {
	for _, prefix := range []string{chromiumPrefix, headlessShellPrefix} {
		found := newestInstall(folder, prefix)
		if found != "" {
			return found
		}
	}
	return ""
}

func newestInstall(folder string, prefix string) string {
	installs, _ := filepath.Glob(filepath.Join(folder, prefix+"*"))
	sort.Slice(installs, func(left int, right int) bool {
		return playwrightRevision(installs[left]) > playwrightRevision(installs[right])
	})
	for _, install := range installs {
		found := chromeInsideInstall(install)
		if found != "" {
			return found
		}
	}
	return ""
}

func playwrightRevision(install string) int {
	name := strings.TrimPrefix(filepath.Base(install), headlessShellPrefix)
	revision, err := strconv.Atoi(strings.TrimPrefix(name, chromiumPrefix))
	if err != nil {
		return -1
	}
	return revision
}

func chromeInsideInstall(install string) string {
	patterns := []string{
		filepath.Join(install, "chrome-mac*", "*.app", "Contents", "MacOS", "*"),
		filepath.Join(install, "chrome-linux*", "chrome"),
		filepath.Join(install, "chrome-win*", "chrome.exe"),
		filepath.Join(install, "chrome-headless-shell-*", "chrome-headless-shell"),
		filepath.Join(install, "chrome-headless-shell-*", "chrome-headless-shell.exe"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

func StartBrowser(chromePath string) (*Browser, error) {
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.WindowSize(2560, 1440),
	)
	allocatorContext, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	if err := chromedp.Do(browserContext); err != nil {
		cancelBrowser()
		cancelAllocator()
		return nil, err
	}
	return &Browser{Context: browserContext, cancelBrowser: cancelBrowser, cancelAllocator: cancelAllocator}, nil
}

func (browser *Browser) Close() {
	browser.cancelBrowser()
	browser.cancelAllocator()
}
