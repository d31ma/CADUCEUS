package search

import "mesail.dev/packages/textindex"

type Filter struct {
	Terms          []string
	ConversationId string
	Author         string
	From           string
	To             string
	HasAttachment  *bool
	IsEmail        *bool
	Cursor         textindex.Cursor
	Limit          int
}
