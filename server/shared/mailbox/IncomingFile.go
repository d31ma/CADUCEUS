import (
	"os"
	"path/filepath"
)

// IncomingFile stages bytes in a private temporary directory under FYLO's
// incoming root, hands the file's path to a store step and removes the
// directory afterwards, whatever the step answered.
type IncomingFile struct {
	Prefix string
	Name   string
	Mode   os.FileMode
}

// Stored writes content, runs store with the staged path and answers its id.
func (file IncomingFile) Stored(fyloRoot string, content []byte, store func(path string) (string, error)) (string, error) {
	incomingRoot, err := filepath.Abs(file.root(fyloRoot))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(incomingRoot, 0o700); err != nil {
		return "", err
	}
	temporaryRoot, err := os.MkdirTemp(incomingRoot, file.Prefix)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temporaryRoot)
	path := filepath.Join(temporaryRoot, file.Name)
	if err := os.WriteFile(path, content, file.Mode); err != nil {
		return "", err
	}
	return store(path)
}

func (IncomingFile) root(fyloRoot string) string {
	if configured := (Settings{}).Value("FYLO_INCOMING_ROOT"); configured != "" {
		return configured
	}
	return filepath.Join(fyloRoot, ".caduceus-incoming")
}
