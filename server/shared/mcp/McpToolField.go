// McpToolField is one argument of an MCP tool, as its zod schema declared
// it: the kind (string, integer, boolean or array), the bounds, a default or
// whether it may be left out, a string format and the item of an array.
type McpToolField struct {
	Name     string
	Kind     string
	Format   string
	Minimum  *int
	Maximum  *int
	Default  any
	Optional bool
	Items    *McpToolField
}
