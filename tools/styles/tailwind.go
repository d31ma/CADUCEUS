package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	tailwindVersion = "4.3.5"
	releaseURL      = "https://github.com/tailwindlabs/tailwindcss/releases/download/v" + tailwindVersion + "/"
	sourceName      = "classes.txt"
	inputCSS        = `@import "tailwindcss/theme.css" reference;
@import "tailwindcss/utilities.css" layer(utilities) source(none);

@theme reference {
  --color-*: initial;
  --font-*: initial;
  --font-weight-*: initial;
  --text-*: initial;
  --leading-*: initial;
  --tracking-*: initial;
  --shadow-*: initial;
  --inset-shadow-*: initial;
  --drop-shadow-*: initial;
  --text-shadow-*: initial;

  --breakpoint-*: initial;
  --breakpoint-sm: 600px;
  --breakpoint-md: 960px;
  --breakpoint-lg: 1280px;
  --breakpoint-xl: 1920px;
  --breakpoint-2xl: 2560px;

  --container-*: initial;
  --container-xs: 20rem;
  --container-sm: 24rem;
  --container-md: 28rem;
  --container-lg: 32rem;
  --container-xl: 40rem;
  --container-2xl: 42rem;
  --container-3xl: 48rem;
  --container-4xl: 56rem;
  --container-5xl: 64rem;
  --container-6xl: 72rem;
  --container-7xl: 80rem;
}

@theme inline reference {
  --font-sans: var(--w-font-family);
  --font-mono: var(--w-font-family-mono);
  --color-surface: var(--w-surface);
  --color-surface-low: var(--w-surface-container-low);
  --color-surface-mid: var(--w-surface-container);
  --color-surface-high: var(--w-surface-container-high);
  --color-sidebar: var(--w-sidebar);
  --color-text: var(--w-text);
  --color-subtle: var(--w-text-subtle);
  --color-very-subtle: var(--w-text-very-subtle);
  --color-divider: var(--w-divider);
  --color-border: var(--w-border);
  --color-outline: var(--w-outline);
  --color-hover: var(--w-hover);
  --color-selected: var(--w-selected);
  --color-on-selected: var(--w-selected-text);
  --color-accent: var(--w-accent);
  --color-accent-bg: var(--w-accent-bg);
  --color-on-accent: var(--w-on-accent);
  --color-primary-container: var(--w-primary-container);
  --color-on-primary-container: var(--w-on-primary-container);
  --color-error: var(--w-error);
  --color-error-container: var(--w-error-container);
  --color-on-error-container: var(--w-on-error-container);
  --color-warning: var(--w-warning);
  --color-success: var(--w-success);
  --color-warning-container: var(--w-warning-container);
  --color-on-warning-container: var(--w-on-warning-container);
  --color-success-container: var(--w-success-container);
  --color-on-success-container: var(--w-on-success-container);
  --color-scrim: var(--w-scrim);
  --color-inverse: var(--w-inverse-surface);
  --color-on-inverse: var(--w-inverse-on-surface);
  --color-chat: var(--ms-chat);
  --color-chat-bg: var(--ms-chat-bg);
  --color-on-chat-bg: var(--ms-on-chat-bg);
  --color-email: var(--ms-email);
  --color-email-bg: var(--ms-email-bg);
  --color-on-email-bg: var(--ms-on-email-bg);
  --color-email-rule: var(--ms-email-rule);
  --color-presence-active: var(--ms-presence-active);
  --color-presence-idle: var(--ms-presence-idle);
  --color-presence-dnd: var(--ms-presence-dnd);
  --color-presence-offline: var(--ms-presence-offline);
  --color-rail: var(--ms-rail-bg);
  --color-on-rail: var(--ms-on-rail);
  --color-msg-hover: var(--ms-msg-hover);
  --color-msg-highlight: var(--ms-msg-highlight);
  --radius-sm: var(--w-radius);
  --radius-md: var(--w-radius-md);
  --radius-lg: var(--w-radius-lg);
  --radius-xl: var(--w-radius-xl);
  --text-xs: var(--w-font-xs);
  --text-xs--line-height: calc(1 / 0.75);
  --text-sm: var(--w-font-base);
  --text-sm--line-height: calc(1.25 / 0.875);
  --text-base: var(--w-font-md);
  --text-base--line-height: calc(1.5 / 1);
  --text-lg: var(--w-font-lg);
  --text-lg--line-height: calc(1.75 / 1.125);
  --text-xl: var(--w-font-xl);
  --text-xl--line-height: calc(1.75 / 1.25);
  --text-2xl: var(--w-font-2xl);
  --text-2xl--line-height: calc(2 / 1.5);
  --font-weight-thin: var(--w-font-weight-thin);
  --font-weight-light: var(--w-font-weight-light);
  --font-weight-normal: var(--w-font-weight-normal);
  --font-weight-medium: var(--w-font-weight-medium);
  --font-weight-semibold: var(--w-font-weight-semibold);
  --font-weight-bold: var(--w-font-weight-bold);
  --font-weight-black: var(--w-font-weight-black);
  --tracking-tight: var(--w-letter-spacing-tight);
  --tracking-normal: var(--w-letter-spacing-normal);
  --tracking-wide: var(--w-letter-spacing-wide);
  --leading-snug: var(--w-line-height-snug);
  --leading-normal: var(--w-line-height-normal);
  --leading-relaxed: var(--w-line-height-relaxed);
  --shadow-sm: var(--w-shadow-sm);
  --shadow-md: var(--w-shadow-md);
  --shadow-lg: var(--w-shadow-lg);
}

@utility safe-t { padding-top: max(env(safe-area-inset-top, 0px), var(--tac-titlebar-height, 0px)); }
@utility safe-b { padding-bottom: env(safe-area-inset-bottom, 0px); }
@utility safe-l { padding-left: max(var(--ms-edge, 0px), env(safe-area-inset-left, 0px), var(--tac-titlebar-leading-inset, 0px)); }
@utility safe-r { padding-right: max(var(--ms-edge, 0px), env(safe-area-inset-right, 0px), var(--tac-titlebar-trailing-inset, 0px)); }
@utility touch-target { min-width: var(--w-touch-min, 2.75rem); min-height: var(--w-touch-min, 2.75rem); }
@utility drag-region { -webkit-app-region: drag; app-region: drag; }
@utility no-drag { -webkit-app-region: no-drag; app-region: no-drag; }

@custom-variant short (@media (max-height: 520px));

@source "./` + sourceName + `";
`
)

type tailwind struct {
	path string
}

func assetName(system, architecture string) (string, error) {
	platforms := map[string]string{"darwin": "macos", "linux": "linux"}
	machines := map[string]string{"arm64": "arm64", "amd64": "x64"}
	platform, knownPlatform := platforms[system]
	machine, knownMachine := machines[architecture]
	if !knownPlatform || !knownMachine {
		return "", fmt.Errorf("no pinned Tailwind %s binary for %s/%s", tailwindVersion, system, architecture)
	}
	return "tailwindcss-" + platform + "-" + machine, nil
}

func assetChecksum(name string) string {
	checksums := map[string]string{
		"tailwindcss-macos-arm64": "f3f4f5d1727e2b693b8f1c9a1492b5dd87008c86ea286bee1589cb458e6235fc",
		"tailwindcss-macos-x64":   "a63702686a3ed048b3135d84a73be49ea9d47f880c9b20e54b3ef60934ad8cdf",
		"tailwindcss-linux-arm64": "ffcd0d21dbd7c605dc12743ff0c0b47debc963c86b399674084c1c8f2c96ecad",
		"tailwindcss-linux-x64":   "e47695cee3f0d90623a91f3939b08bdee610eb5ad99340adff9d4145b284d6cc",
	}
	return checksums[name]
}

func cacheDirectory() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		directory, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		base = directory
	}
	return filepath.Join(base, "mesail-tools", "tailwindcss-"+tailwindVersion), nil
}

func ensureTailwind(log io.Writer) (tailwind, error) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return tailwind{}, err
	}
	directory, err := cacheDirectory()
	if err != nil {
		return tailwind{}, err
	}
	path := filepath.Join(directory, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := download(log, name, path); err != nil {
			return tailwind{}, err
		}
	}
	if err := verify(path, assetChecksum(name)); err != nil {
		return tailwind{}, fmt.Errorf("%w; delete %s to download it again", err, path)
	}
	return tailwind{path: path}, nil
}

func download(log io.Writer, name, path string) error {
	fmt.Fprintf(log, "styles: downloading %s%s to %s\n", releaseURL, name, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	response, err := http.Get(releaseURL + name)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %s", name, response.Status)
	}
	partial := path + ".part"
	if err := writeExecutable(partial, response.Body); err != nil {
		return err
	}
	if err := verify(partial, assetChecksum(name)); err != nil {
		os.Remove(partial)
		return err
	}
	return os.Rename(partial, path)
}

func writeExecutable(path string, body io.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, body); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func verify(path, expected string) error {
	actual, err := fileChecksum(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%s has SHA-256 %s, expected %s; refusing to run it", path, actual, expected)
	}
	return nil
}

func fileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (binary tailwind) compile(template []byte) (string, error) {
	folder, err := os.MkdirTemp("", "mesail-styles-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(folder)
	if err := os.WriteFile(filepath.Join(folder, sourceName), []byte(classSource(template)), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(folder, "input.css"), []byte(inputCSS), 0o644); err != nil {
		return "", err
	}
	var output, errors bytes.Buffer
	command := exec.Command(binary.path, "--input", "input.css", "--output", "-")
	command.Dir = folder
	command.Stdout = &output
	command.Stderr = &errors
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("tailwindcss: %v: %s", err, errors.String())
	}
	return output.String(), nil
}
