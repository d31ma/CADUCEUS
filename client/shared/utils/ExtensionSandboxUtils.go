import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ExtensionSandboxUtils builds what the extension runtime puts on the page,
// as shared/scripts/extension-runtime.js did: the opaque sandbox document a
// plugin runs in, the CSS variables of a declarative theme, and the
// permission-filtered notification of a command result.
type ExtensionSandboxUtils struct{}

// document is a plugin's iframe srcdoc. Its only script starts a classic Web
// Worker from an in-memory Blob, so the package source never runs in the
// application document or in the iframe's own thread; the source travels
// base64-encoded and never appears in the document as text.
func (ExtensionSandboxUtils) document(extensionID string, source string, nonce string) string {
	utils := ExtensionSandboxUtils{}
	worker := base64.StdEncoding.EncodeToString([]byte(utils.workerSource(source)))
	timeout := strconv.Itoa(utils.commandTimeoutMs())
	bootstrap := `
const extensionId = ` + utils.literal(utils.bounded(extensionID, 128)) + `;
const sessionNonce = ` + utils.literal(nonce) + `;
const workerBytes = Uint8Array.from(atob(` + utils.literal(worker) + `), character => character.charCodeAt(0));
const worker = new Worker(URL.createObjectURL(new Blob([workerBytes], { type: "text/javascript" })));
const pending = new Map();
const send = message => parent.postMessage({ channel: "caduceus-extension", extensionId, sessionNonce, ...message }, "*");
const startupTimeout = setTimeout(() => { worker.terminate(); send({ type: "error", error: "Extension activation timed out" }); }, ` + timeout + `);
worker.onmessage = event => {
  const message = event.data || {};
  let serialized = "";
  try { serialized = JSON.stringify(message); } catch { worker.terminate(); send({ type: "error", error: "Extension returned an invalid result" }); return; }
  if (serialized.length > ` + strconv.Itoa(utils.maxResultBytes()) + `) { worker.terminate(); send({ type: "error", error: "Extension result exceeded 16 KiB" }); return; }
  if (message.type === "ready") clearTimeout(startupTimeout);
  if (message.requestId) { clearTimeout(pending.get(message.requestId)); pending.delete(message.requestId); }
  send(message);
};
worker.onerror = () => { clearTimeout(startupTimeout); worker.terminate(); send({ type: "error", error: "Extension worker failed" }); };
addEventListener("message", event => {
  const message = event.data || {};
  if (event.source !== parent || message.channel !== "caduceus-extension-host" || message.sessionNonce !== sessionNonce || message.extensionId !== extensionId || message.type !== "command") return;
  const timeout = setTimeout(() => {
    worker.terminate(); pending.delete(message.requestId);
    send({ type: "error", requestId: message.requestId, error: "Extension command timed out" });
  }, ` + timeout + `);
  pending.set(message.requestId, timeout);
  worker.postMessage({ type: "command", requestId: message.requestId, command: message.command, context: message.context });
});`
	policy := "default-src 'none'; script-src 'nonce-" + nonce + "'; worker-src blob:; connect-src 'none'; img-src 'none'; style-src 'none'; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'"
	return `<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="` + policy + `"><script nonce="` + nonce + `">` + bootstrap + `</script>`
}

// ThemeVariables maps a theme's colour tokens onto the fixed CSS variables a
// theme may set; a token that is not a hex colour is dropped.
func (ExtensionSandboxUtils) ThemeVariables(theme ExtensionTheme) map[string]string {
	variables := map[string]string{}
	color := regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	for token, variable := range (ExtensionSandboxUtils{}).themeVariables() {
		value := theme.Tokens[token]
		if color.MatchString(value) {
			variables[variable] = strings.ToLower(value)
		}
	}
	return variables
}

// Notification is the result's notify text when the plugin holds ui:notify,
// trimmed and bounded, or "".
func (ExtensionSandboxUtils) Notification(permissions []string, result string) string {
	answer := map[string]any{}
	if len(result) > (ExtensionSandboxUtils{}).maxResultBytes() || json.Unmarshal([]byte(result), &answer) != nil {
		return ""
	}
	notify, _ := answer["notify"].(string)
	if !slices.Contains(permissions, "ui:notify") {
		return ""
	}
	return ExtensionSandboxUtils{}.bounded(strings.TrimSpace(notify), ExtensionSandboxUtils{}.noticeLength())
}

// Reply reads what a sandbox posted (as JSON text) into its fields.
func (ExtensionSandboxUtils) Reply(message string) ExtensionReply {
	fields := map[string]any{}
	_ = json.Unmarshal([]byte(message), &fields)
	text := func(name string) string {
		value, _ := fields[name].(string)
		return value
	}
	result, _ := json.Marshal(fields["result"])
	return ExtensionReply{
		ExtensionID: text("extensionId"), Nonce: text("sessionNonce"), Kind: text("type"),
		RequestID: text("requestId"), Result: string(result), Error: text("error"),
	}
}

// EnabledTheme is the theme of the enabled theme extension, or no theme.
func (ExtensionSandboxUtils) EnabledTheme(extensions []InstalledExtension) ExtensionTheme {
	for _, extension := range extensions {
		if extension.Enabled && extension.Kind == "theme" {
			return extension.Manifest.Theme
		}
	}
	return ExtensionTheme{}
}

// EnabledPlugins are the enabled plugin extensions, in the server's order.
func (ExtensionSandboxUtils) EnabledPlugins(extensions []InstalledExtension) []InstalledExtension {
	plugins := []InstalledExtension{}
	for _, extension := range extensions {
		if extension.Enabled && extension.Kind == "plugin" {
			plugins = append(plugins, extension)
		}
	}
	return plugins
}

// commands are a plugin's command buttons, disabled until its sandbox says
// it is ready.
func (ExtensionSandboxUtils) commands(extension InstalledExtension, ready bool) []ExtensionCommand {
	commands := []ExtensionCommand{}
	for _, contribution := range extension.Manifest.Contributions {
		commands = append(commands, ExtensionCommand{
			ExtensionID: extension.ID, CommandID: contribution.ID, Label: contribution.Label, Slot: contribution.Slot,
			Title: extension.DisplayName + ": " + contribution.Label, Disabled: !ready,
		})
	}
	return commands
}

// Toolbar is every plugin's command buttons.
func (ExtensionSandboxUtils) Toolbar(plugins []InstalledExtension, ready map[string]bool) []ExtensionCommand {
	commands := []ExtensionCommand{}
	for _, plugin := range plugins {
		commands = append(commands, ExtensionSandboxUtils{}.commands(plugin, ready[plugin.ID])...)
	}
	return commands
}

// Permissions are what the plugin with this id was granted.
func (ExtensionSandboxUtils) Permissions(plugins []InstalledExtension, extensionID string) []string {
	for _, plugin := range plugins {
		if plugin.ID == extensionID {
			return plugin.Manifest.Permissions
		}
	}
	return []string{}
}

// Failure is the toast for a sandbox error: at start, or for one command.
func (ExtensionSandboxUtils) Failure(requestID string, failure string) string {
	if requestID == "" {
		return "Extension failed to start."
	}
	return "Extension failed: " + ExtensionSandboxUtils{}.bounded(failure, ExtensionSandboxUtils{}.noticeLength())
}

// Sandbox is a plugin's iframe.
func (ExtensionSandboxUtils) Sandbox(extension InstalledExtension, nonce string) ExtensionSandbox {
	name := ExtensionSandboxUtils{}.bounded(extension.DisplayName, 80)
	if name == "" {
		name = extension.ID
	}
	return ExtensionSandbox{ID: extension.ID, Title: name + " extension sandbox", Document: ExtensionSandboxUtils{}.document(extension.ID, extension.Source, nonce)}
}

func (ExtensionSandboxUtils) workerSource(source string) string {
	return `"use strict";
let commandHandler = null;
const caduceus = Object.freeze({
  activate(candidate) {
    if (!candidate || typeof candidate.onCommand !== "function") throw new Error("Extension must register onCommand");
    if (commandHandler) throw new Error("Extension may activate only once");
    commandHandler = candidate.onCommand;
  }
});
globalThis.caduceus = caduceus;
self.onmessage = async event => {
  const message = event.data || {};
  if (message.type !== "command" || !commandHandler) return;
  try {
    const result = await commandHandler(Object.freeze(message.command), Object.freeze(message.context));
    self.postMessage({ type: "result", requestId: message.requestId, result });
  } catch (error) {
    self.postMessage({ type: "error", requestId: message.requestId, error: String(error && error.message || error) });
  }
};
try {
` + source + `
  if (!commandHandler) throw new Error("Extension did not activate");
  self.postMessage({ type: "ready" });
} catch (error) {
  self.postMessage({ type: "error", error: String(error && error.message || error) });
}`
}

func (ExtensionSandboxUtils) themeVariables() map[string]string {
	return map[string]string{
		"danger":                 "--ms-error",
		"on-primary":             "--ms-on-primary",
		"primary":                "--ms-primary",
		"primary-container":      "--ms-primary-container",
		"surface":                "--ms-surface",
		"surface-container":      "--ms-surface-container",
		"surface-container-high": "--ms-surface-container-high",
		"text":                   "--ms-on-surface",
		"text-subtle":            "--ms-on-surface-variant",
	}
}

func (ExtensionSandboxUtils) literal(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func (ExtensionSandboxUtils) bounded(value string, maximum int) string {
	runes := []rune(value)
	return string(runes[:min(len(runes), maximum)])
}

func (ExtensionSandboxUtils) commandTimeoutMs() int { return 2000 }

func (ExtensionSandboxUtils) noticeLength() int { return 200 }

func (ExtensionSandboxUtils) maxResultBytes() int { return 16 * 1024 }
