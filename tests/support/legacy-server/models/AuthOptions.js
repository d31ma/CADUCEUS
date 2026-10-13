/**
 * @typedef {object} AuthOptions
 * @property {string} [action] - Action to authorize. Omit to require only a
 *   live session.
 * @property {(request: HandlerRequest) => string | undefined} [mailbox] -
 *   Resolves the mailbox the action applies to. Defaults to the caller's own.
 * @property {(request: HandlerRequest) => string | undefined} [domain] -
 *   Resolves the domain the action applies to.
 */
export {}
