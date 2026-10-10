// McpPrincipal is the mailbox an MCP capability acts for and the scope its
// owner allowed: the access level, the domains and the excluded folders.
type McpPrincipal struct {
	OwnerEmail      string
	TokenId         string
	PrimaryDomain   string
	AccessLevel     string
	AllowedDomains  []string
	ExcludedFolders map[string][]string
}
