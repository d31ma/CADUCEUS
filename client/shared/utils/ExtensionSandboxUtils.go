import (
	"cmp"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ExtensionSandboxUtils builds what the extension runtime puts on the page,
// as shared/scripts/extension-runtime.js did: the opaque sandbox document the
// plugins run in, the CSS variables of a declarative theme, and the
// permission-filtered notification of a command result.
type ExtensionSandboxUtils struct{}

// HostDocument is the srcdoc of the one opaque sandbox iframe that hosts
// every enabled plugin. Its only script starts each plugin as a classic Web
// Worker from an in-memory Blob when the page sends the plugin's source, so
// package source never runs in the application document or the iframe's own
// thread, and never appears in the document. The iframe stays the same
// while plugins come and go, so a page render never restarts them.
func (ExtensionSandboxUtils) HostDocument(nonce string) string {
	utils := ExtensionSandboxUtils{}
	marker := utils.sourceMarker()
	prefix, suffix, _ := strings.Cut(utils.workerSource(marker), marker)
	timeout := strconv.Itoa(utils.commandTimeoutMs())
	bootstrap := `
const prefix = ` + utils.literal(prefix) + `;
const suffix = ` + utils.literal(suffix) + `;
const workers = new Map();
const send = (extensionId, sessionNonce, message) => parent.postMessage({ channel: "caduceus-extension", extensionId, sessionNonce, ...message }, "*");
const stop = extensionId => {
  const entry = workers.get(extensionId);
  if (!entry) return;
  clearTimeout(entry.startupTimeout);
  entry.pending.forEach(timer => clearTimeout(timer));
  entry.worker.terminate();
  URL.revokeObjectURL(entry.sourceUrl);
  workers.delete(extensionId);
};
function load(extensionId, sessionNonce, source) {
  stop(extensionId);
  const sourceUrl = URL.createObjectURL(new Blob([prefix, source, suffix], { type: "text/javascript" }));
  let worker;
  try { worker = new Worker(sourceUrl); }
  catch { URL.revokeObjectURL(sourceUrl); send(extensionId, sessionNonce, { type: "stopped", error: "Extension worker failed to start" }); return; }
  const pending = new Map();
  const startupTimeout = setTimeout(() => { stop(extensionId); send(extensionId, sessionNonce, { type: "stopped", error: "Extension activation timed out" }); }, ` + timeout + `);
  workers.set(extensionId, { worker, sessionNonce, pending, startupTimeout, sourceUrl });
  worker.onmessage = event => {
    const message = event.data || {};
    let serialized = "";
    try { serialized = JSON.stringify(message); } catch { stop(extensionId); send(extensionId, sessionNonce, { type: "stopped", error: "Extension returned an invalid result" }); return; }
    if (serialized.length > ` + strconv.Itoa(utils.maxResultBytes()) + `) { stop(extensionId); send(extensionId, sessionNonce, { type: "stopped", error: "Extension result exceeded 16 KiB" }); return; }
    if (message.type === "ready") clearTimeout(startupTimeout);
    if (message.type === "error" && !message.requestId) { stop(extensionId); send(extensionId, sessionNonce, { ...message, type: "stopped" }); return; }
    if (message.requestId) { clearTimeout(pending.get(message.requestId)); pending.delete(message.requestId); }
    send(extensionId, sessionNonce, message);
  };
  worker.onerror = () => { stop(extensionId); send(extensionId, sessionNonce, { type: "stopped", error: "Extension worker failed" }); };
}
addEventListener("message", event => {
  const message = event.data || {};
  if (event.source !== parent || message.channel !== "caduceus-extension-host") return;
  if (message.type === "reset") { [...workers.keys()].forEach(stop); return; }
  if (message.type === "load") { load(String(message.extensionId), String(message.sessionNonce), String(message.source || "")); return; }
  const entry = workers.get(message.extensionId);
  if (!entry || message.sessionNonce !== entry.sessionNonce || message.type !== "command") return;
  const timeout = setTimeout(() => {
    stop(message.extensionId);
    send(message.extensionId, entry.sessionNonce, { type: "stopped", requestId: message.requestId, error: "Extension command timed out" });
  }, ` + timeout + `);
  entry.pending.set(message.requestId, timeout);
  entry.worker.postMessage({ type: "command", requestId: message.requestId, command: message.command, context: message.context });
});
parent.postMessage({ channel: "caduceus-extension-host-ready" }, "*");`
	policy := "default-src 'none'; script-src 'nonce-" + nonce + "'; worker-src blob:; connect-src 'none'; img-src 'none'; style-src 'none'; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'"
	return `<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="` + policy + `"><script nonce="` + nonce + `">` + bootstrap + `</script>`
}

// Title names the plugins the sandbox hosts, for assistive technology.
func (ExtensionSandboxUtils) Title(plugins []InstalledExtension) string {
	return ExtensionSandboxUtils{}.title(plugins) + " extension sandbox"
}

// ThemeVariables maps a theme's colour tokens onto the fixed CSS variables a
// theme may set; a token that is not a hex colour is dropped.
func (ExtensionSandboxUtils) ThemeVariables(theme ExtensionTheme) map[string]string {
	variables := ExtensionSandboxUtils{}.themeColors(theme)
	color := regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	maps.DeleteFunc(variables, func(_ string, value string) bool { return !color.MatchString(value) })
	return variables
}

// Notification is the result's notify text when the plugin holds ui:notify,
// trimmed and bounded, or "".
func (ExtensionSandboxUtils) Notification(permissions []string, result string) string {
	answer := map[string]any{}
	parsed := json.Unmarshal([]byte(result), &answer) == nil
	notify, _ := answer["notify"].(string)
	allowed := ChoiceUtils{}.All(len(result) <= ExtensionSandboxUtils{}.maxResultBytes(), parsed, slices.Contains(permissions, "ui:notify"))
	return ChoiceUtils{}.Text(allowed, ExtensionSandboxUtils{}.bounded(strings.TrimSpace(notify), ExtensionSandboxUtils{}.noticeLength()), "")
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
	index := slices.IndexFunc(extensions, func(extension InstalledExtension) bool {
		return ChoiceUtils{}.All(extension.Enabled, extension.Kind == "theme")
	})
	return ExtensionSandboxUtils{}.at(extensions, index).Manifest.Theme
}

// EnabledPlugins are the enabled plugin extensions, in the server's order.
func (ExtensionSandboxUtils) EnabledPlugins(extensions []InstalledExtension) []InstalledExtension {
	return slices.DeleteFunc(slices.Clone(extensions), func(extension InstalledExtension) bool {
		return !ChoiceUtils{}.All(extension.Enabled, extension.Kind == "plugin")
	})
}

// Toolbar is every plugin's command buttons, each disabled until its
// sandbox says it is ready.
func (ExtensionSandboxUtils) Toolbar(plugins []InstalledExtension, ready map[string]bool) []ExtensionCommand {
	return ExtensionSandboxUtils{}.toolbar(plugins, ready)
}

// Permissions are what the plugin with this id was granted.
func (ExtensionSandboxUtils) Permissions(plugins []InstalledExtension, extensionID string) []string {
	index := slices.IndexFunc(plugins, func(plugin InstalledExtension) bool { return plugin.ID == extensionID })
	return ExtensionSandboxUtils{}.at(plugins, index).Manifest.Permissions
}

// Failure is the toast for a sandbox error: at start, or for one command.
func (ExtensionSandboxUtils) Failure(requestID string, failure string) string {
	command := "Extension failed: " + ExtensionSandboxUtils{}.bounded(failure, ExtensionSandboxUtils{}.noticeLength())
	return ChoiceUtils{}.Text(requestID == "", "Extension failed to start.", command)
}

// Sandbox is what the bridge sends the sandbox host to start one plugin.
func (ExtensionSandboxUtils) Sandbox(extension InstalledExtension, nonce string) ExtensionSandbox {
	return ExtensionSandbox{ID: extension.ID, Nonce: nonce, Source: extension.Source}
}

func (ExtensionSandboxUtils) title(plugins []InstalledExtension) string {
	names := []string{}
	for _, plugin := range plugins {
		names = append(names, cmp.Or(ExtensionSandboxUtils{}.bounded(plugin.DisplayName, 80), plugin.ID))
	}
	return strings.Join(names, ", ")
}

func (ExtensionSandboxUtils) themeColors(theme ExtensionTheme) map[string]string {
	variables := map[string]string{}
	for token, variable := range (ExtensionSandboxUtils{}).themeVariables() {
		variables[variable] = strings.ToLower(theme.Tokens[token])
	}
	return variables
}

// at is the extension at index, or an empty one when index is -1.
func (ExtensionSandboxUtils) at(extensions []InstalledExtension, index int) InstalledExtension {
	padded := append(slices.Clone(extensions), InstalledExtension{})
	return padded[map[bool]int{true: index, false: len(extensions)}[index >= 0]]
}

func (ExtensionSandboxUtils) toolbar(plugins []InstalledExtension, ready map[string]bool) []ExtensionCommand {
	commands := []ExtensionCommand{}
	for _, plugin := range plugins {
		commands = append(commands, ExtensionCommandUtils{}.Commands(plugin, ready[plugin.ID])...)
	}
	return commands
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

// sourceMarker stands where the plugin source goes in the worker program.
func (ExtensionSandboxUtils) sourceMarker() string { return "\x00" }

func (ExtensionSandboxUtils) maxResultBytes() int { return 16 * 1024 }
