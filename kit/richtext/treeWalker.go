package richtext

import "golang.org/x/net/html"

type treeWalker struct {
	enter func(*html.Node) bool
	leave func(*html.Node)
}

func (walker treeWalker) walk(root *html.Node) {
	node := root
	for node != nil {
		node = walker.step(node)
	}
}

func (walker treeWalker) step(node *html.Node) *html.Node {
	descend := walker.enter(node)
	return walker.following(node, descend)
}

func (walker treeWalker) following(node *html.Node, descend bool) *html.Node {
	if descend && node.FirstChild != nil {
		return node.FirstChild
	}
	return walker.climb(node)
}

func (walker treeWalker) climb(node *html.Node) *html.Node {
	for node.NextSibling == nil && node.Parent != nil {
		walker.leave(node)
		node = node.Parent
	}
	walker.leave(node)
	return node.NextSibling
}
