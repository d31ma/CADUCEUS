export class McpPreferenceError extends Error {
  constructor(message, forbidden = false) { super(message); this.forbidden = forbidden }
}
