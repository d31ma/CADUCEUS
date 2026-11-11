package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	templateName = "tac.html"
	styleName    = "tac.css"
	workers      = 4
)

type component struct {
	folder        string
	rules         string
	registrations map[string]string
}

type compiler interface {
	compile(template []byte) (string, error)
}

func findComponents(root string) ([]string, error) {
	folders := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == templateName {
			folders = append(folders, filepath.Dir(path))
		}
		return nil
	})
	sort.Strings(folders)
	return folders, err
}

func buildComponents(tool compiler, folders []string) ([]component, error) {
	results := make([]component, len(folders))
	failures := make([]error, len(folders))
	slots := make(chan struct{}, workers)
	var group sync.WaitGroup
	for index, folder := range folders {
		group.Add(1)
		slots <- struct{}{}
		go func() {
			defer group.Done()
			results[index], failures[index] = buildComponent(tool, folder)
			<-slots
		}()
	}
	group.Wait()
	return results, errors.Join(failures...)
}

func buildComponent(tool compiler, folder string) (component, error) {
	template, err := os.ReadFile(filepath.Join(folder, templateName))
	if err != nil {
		return component{}, err
	}
	css, err := tool.compile(template)
	if err != nil {
		return component{}, fmt.Errorf("%s: %w", folder, err)
	}
	rules, registrations := splitRegistrations(css)
	if colors := hardCodedColors(rules); len(colors) > 0 {
		return component{}, fmt.Errorf("%s: Tailwind classes produce hard-coded colors (use DuVay tokens): %s", folder, strings.Join(colors, " "))
	}
	if !hasRules(rules) {
		rules = ""
	}
	return component{folder: folder, rules: rules, registrations: registrations}, nil
}
