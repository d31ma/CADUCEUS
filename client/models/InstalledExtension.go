type InstalledExtension struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	Version     string            `json:"version"`
	Kind        string            `json:"kind"`
	Digest      string            `json:"digest"`
	Enabled     bool              `json:"enabled"`
	Manifest    ExtensionManifest `json:"manifest"`
	Source      string            `json:"source"`
}
