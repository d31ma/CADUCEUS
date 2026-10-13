import (
	"encoding/json"
	"os"
	"strings"
)

// Settings reads one CADUCEUS setting by its environment name, such as
// FYLO_ROOT. A Tachyon handler receives no application variables; Tachyon
// hands it the yon.* configuration from server/application.json and the
// project .env as YON_APPLICATION_JSON, where FYLO_ROOT is yon.fylo.root.
// A process that CADUCEUS starts itself reads the variable directly.
type Settings struct{}

// Value answers the setting, or the empty text when it is not set.
func (settings Settings) Value(name string) string {
	if direct, found := os.LookupEnv(name); found {
		return direct
	}
	return settings.configured(name)
}

func (settings Settings) configured(name string) string {
	var node any = settings.yon()
	for _, key := range strings.Split(strings.ToLower(name), "_") {
		node = settings.child(node, key)
	}
	return settings.text(node)
}

func (Settings) yon() map[string]any {
	var document struct {
		Yon map[string]any `json:"yon"`
	}
	_ = json.Unmarshal([]byte(os.Getenv("YON_APPLICATION_JSON")), &document)
	return document.Yon
}

func (Settings) child(node any, key string) any {
	object, isObject := node.(map[string]any)
	if !isObject {
		return nil
	}
	return object[key]
}

func (Settings) text(node any) string {
	switch value := node.(type) {
	case string:
		return value
	case float64, bool:
		encoded, _ := json.Marshal(value)
		return string(encoded)
	default:
		return ""
	}
}
