package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const foundationPath = "shared/styles/app.css"

type options struct {
	app   string
	root  string
	check bool
}

type change struct {
	path    string
	content string
}

var errStale = errors.New("generated styles are stale")

func main() {
	settings := options{}
	flag.StringVar(&settings.app, "app", "", "app under apps/ whose client components are styled, for example web")
	flag.StringVar(&settings.root, "root", "", "repository root (default: found from the working directory)")
	flag.BoolVar(&settings.check, "check", false, "regenerate in memory and exit 1 when a generated file is stale")
	flag.Parse()
	err := run(settings, os.Stdout)
	if errors.Is(err, errStale) {
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "styles: %v\n", err)
		os.Exit(2)
	}
}

func run(settings options, log io.Writer) error {
	started := time.Now()
	client, err := clientFolder(settings)
	if err != nil {
		return err
	}
	tool, err := ensureTailwind(log)
	if err != nil {
		return err
	}
	folders, err := findComponents(filepath.Join(client, "components"))
	if err != nil {
		return err
	}
	components, err := buildComponents(tool, folders)
	if err != nil {
		return err
	}
	changes, registrations, err := planChanges(client, components)
	if err != nil {
		return err
	}
	if err := apply(settings.check, client, changes, log); err != nil {
		return err
	}
	fmt.Fprintf(log, "styles: app=%s components=%d registrations=%d changed=%d check=%t seconds=%.2f\n", settings.app, len(components), registrations, len(changes), settings.check, time.Since(started).Seconds())
	return nil
}

func clientFolder(settings options) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(settings.app) {
		return "", fmt.Errorf("--app must name a folder under apps/, got %q", settings.app)
	}
	root, err := repositoryRoot(settings.root)
	if err != nil {
		return "", err
	}
	client := filepath.Join(root, "apps", settings.app, "client")
	if _, err := os.Stat(filepath.Join(client, "components")); err != nil {
		return "", fmt.Errorf("app %q has no client components: %w", settings.app, err)
	}
	return client, nil
}

func repositoryRoot(given string) (string, error) {
	if given != "" {
		return filepath.Abs(given)
	}
	folder, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRoot(folder)
}

func findRoot(folder string) (string, error) {
	for {
		if isRoot(folder) {
			return folder, nil
		}
		parent := filepath.Dir(folder)
		if parent == folder {
			return "", errors.New("repository root not found; pass --root")
		}
		folder = parent
	}
}

func isRoot(folder string) bool {
	_, apps := os.Stat(filepath.Join(folder, "apps"))
	_, tool := os.Stat(filepath.Join(folder, "tools", "styles", "go.mod"))
	return apps == nil && tool == nil
}

func planChanges(client string, components []component) ([]change, int, error) {
	changes := []change{}
	registrations := map[string]string{}
	for _, item := range components {
		path := filepath.Join(item.folder, styleName)
		planned, err := planFile(path, item.rules, false)
		if err != nil {
			return nil, 0, err
		}
		changes = append(changes, planned...)
		for name, block := range item.registrations {
			registrations[name] = block
		}
	}
	planned, err := planFile(filepath.Join(client, foundationPath), foundationBody(registrations), true)
	return append(changes, planned...), len(registrations), err
}

func planFile(path, generated string, foundation bool) ([]change, error) {
	original, err := readOptional(path, foundation)
	if err != nil {
		return nil, err
	}
	expected, err := withRegion(original, generated)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if foundation {
		expected = withLayerOrder(expected)
	}
	if expected == original {
		return nil, nil
	}
	return []change{{path: path, content: expected}}, nil
}

func readOptional(path string, required bool) (string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !required {
		return "", nil
	}
	return string(content), err
}

func apply(check bool, client string, changes []change, log io.Writer) error {
	verb := "wrote"
	if check {
		verb = "stale"
	}
	for _, item := range changes {
		fmt.Fprintf(log, "styles: %s %s\n", verb, relative(client, item.path))
		if check {
			continue
		}
		if err := os.WriteFile(item.path, []byte(item.content), 0o644); err != nil {
			return err
		}
	}
	if check && len(changes) > 0 {
		fmt.Fprintf(log, "styles: %d stale file(s); run: cd tools/styles && go run . --app <app>\n", len(changes))
		return errStale
	}
	return nil
}

func relative(client, path string) string {
	root := filepath.Dir(filepath.Dir(filepath.Dir(client)))
	name, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return name
}
