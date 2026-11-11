package main

import (
	"path"
	"strings"
)

type StaticPaths struct {
}

func (paths StaticPaths) Check(requestPath string) error {
	return paths.refusedWhen(paths.unsafe(requestPath))
}

func (StaticPaths) unsafe(requestPath string) bool {
	return strings.Contains(requestPath, "..") || strings.ContainsAny(requestPath, "\\\x00")
}

func (StaticPaths) refusedWhen(unsafe bool) error {
	if unsafe {
		return Problems{}.BadPath()
	}
	return nil
}

func (paths StaticPaths) Key(requestPath string) string {
	prefix, rest := paths.split(requestPath)
	return prefix + "/" + paths.withIndex(rest)
}

func (paths StaticPaths) Index(requestPath string) string {
	prefix, _ := paths.split(requestPath)
	return prefix + "/" + paths.index()
}

func (StaticPaths) PageLike(requestPath string, accept string) bool {
	return path.Ext(requestPath) == "" || strings.Contains(accept, "text/html")
}

func (paths StaticPaths) split(requestPath string) (string, string) {
	if requestPath == "/admin" || strings.HasPrefix(requestPath, "/admin/") {
		return "admin", strings.TrimPrefix(strings.TrimPrefix(requestPath, "/admin"), "/")
	}
	return "web", strings.TrimPrefix(requestPath, "/")
}

func (paths StaticPaths) withIndex(rest string) string {
	if rest == "" || strings.HasSuffix(rest, "/") {
		return rest + paths.index()
	}
	return rest
}

func (StaticPaths) index() string {
	return "index.html"
}
