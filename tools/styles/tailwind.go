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
	inputCSS        = `@import "tailwindcss/theme.css" reference;
@import "tailwindcss/utilities.css" layer(utilities) source(none);

@theme reference {
  --color-*: initial;
  --font-*: initial;
  --font-weight-*: initial;
  --text-*: initial;
  --leading-*: initial;
  --tracking-*: initial;
  --radius-*: initial;
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
}

@source "./tac.html";
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
	if err := os.WriteFile(filepath.Join(folder, "tac.html"), template, 0o644); err != nil {
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
