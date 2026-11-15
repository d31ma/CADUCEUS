package main

type FullRunPaths struct{}

func (FullRunPaths) Prefixes() []string {
	return []string{
		".github/",
		"scripts/ci/",
		"scripts/gates.sh",
		"scripts/go-cache.sh",
		"scripts/kit-sync.sh",
		"scripts/vendor/",
		"tools/",
	}
}

func (FullRunPaths) FileNames() []string {
	return []string{"go.mod", "go.sum"}
}
