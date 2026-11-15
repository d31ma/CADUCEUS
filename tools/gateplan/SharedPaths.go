package main

type SharedPaths struct{}

func (SharedPaths) Prefixes() []string {
	return []string{
		".claude/",
		".codex/",
		".dockerignore",
		".env.example",
		".gitignore",
		"Dockerfile",
		"LICENSE",
		"deploy/",
		"docker-compose.yml",
		"edge/",
		"kit/",
		"scripts/floci/",
		"scripts/native/",
		"tests/",
	}
}
