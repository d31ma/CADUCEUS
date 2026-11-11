package richtext

import "golang.org/x/net/html"

type elementRule struct {
	enter func(*html.Node) bool
	leave func(*html.Node)
	block bool
}
