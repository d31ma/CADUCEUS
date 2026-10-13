// @HTTPClient({"setup":"SameOriginSetup"})
type McpClient interface {
	// @GetRequest("/mcp/config")
	Config(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PutRequest("/mcp/config")
	SaveConfig(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
		// @RequestBody
		body BehaviorValue,
	) (BehaviorValue, error)

	// @GetRequest("/mcp/audit")
	Audit(
		// @RequestHeader("Authorization")
		authorization string,
	) ([]McpAuditEvent, error)

	// @DeleteRequest("/mcp/audit")
	ClearAudit(
		// @RequestHeader("Authorization")
		authorization BehaviorValue,
	) (BehaviorValue, error)

	// @PostRequest("/mcp/token")
	RotateToken(
		// @RequestHeader("Authorization")
		authorization string,
	) (McpToken, error)

	// @DeleteRequest("/mcp/token")
	RevokeToken(
		// @RequestHeader("Authorization")
		authorization string,
	) (McpRevocation, error)
}
