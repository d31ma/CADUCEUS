package main

import (
	"path"
	"slices"
	"strings"
)

type ChangedPath string

func (changed ChangedPath) Reach(folders AppFolders) Reach {
	facts := PathFacts{
		Path:           string(changed),
		RunsEverything: changed.startsWithAny(FullRunPaths{}.Prefixes()),
		ModuleFile:     slices.Contains(FullRunPaths{}.FileNames(), path.Base(string(changed))),
		OwnApps:        folders.Named(changed.appName()),
		InApp:          changed.appName() != "",
		KitPackage:     changed.kitPackage(),
		KitApps:        folders.Listing(changed.kitPackage()),
		Responsive:     strings.HasPrefix(string(changed), changed.responsiveFolder()),
		WebApps:        folders.Named(changed.responsiveApp()),
		Docs:           changed.isDocs(),
		Shared:         changed.startsWithAny(SharedPaths{}.Prefixes()),
	}
	return facts.reach()
}

func (changed ChangedPath) startsWithAny(prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(string(changed), prefix) {
			return true
		}
	}
	return false
}

func (changed ChangedPath) appName() string {
	parts := strings.SplitN(string(changed), "/", 3)
	if len(parts) < 3 || parts[0] != "apps" {
		return ""
	}
	return parts[1]
}

func (changed ChangedPath) kitPackage() string {
	inside, found := strings.CutPrefix(path.Dir(string(changed)), "kit/")
	if !found {
		return ""
	}
	return inside
}

func (ChangedPath) responsiveFolder() string {
	return "tests/responsive/"
}

func (ChangedPath) responsiveApp() string {
	return "web"
}

func (changed ChangedPath) isDocs() bool {
	text := string(changed)
	return strings.HasPrefix(text, "docs/") || (!strings.Contains(text, "/") && strings.HasSuffix(text, ".md"))
}
