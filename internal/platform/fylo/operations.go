package fylo

var mutationOperations = map[string]struct{}{
	"backupReconcile":   {},
	"batchPutData":      {},
	"branch":            {},
	"checkout":          {},
	"commit":            {},
	"createCollection":  {},
	"delDoc":            {},
	"delDocs":           {},
	"dropCollection":    {},
	"executeSQL":        {},
	"importBulkData":    {},
	"merge":             {},
	"patchDoc":          {},
	"patchDocs":         {},
	"putData":           {},
	"queueAck":          {},
	"queueExtend":       {},
	"queueNack":         {},
	"queuePublish":      {},
	"rebuildCollection": {},
	"restoreCommit":     {},
	"restoreDoc":        {},
	"schemaMaterialize": {},
	"setMeta":           {},
}

// IsMutation reports whether losing an acknowledgement leaves an uncertain
// durable outcome.
func IsMutation(operation string) bool {
	_, found := mutationOperations[operation]
	return found
}
