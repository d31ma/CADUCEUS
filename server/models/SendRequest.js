/**
 * Outbound send orchestration: immediate delivery, delayed (undo-window) send,
 * and scheduled send — plus persisting the "Sent" copy each path needs for
 * tracking. Boundary validation (rate limit, request shape, suppression) stays
 * in the `/send` controller; this service owns what happens once a send is
 * accepted.
 *
 * @typedef {import('./smtp.js').SendRequest} SendRequest
 */
export {}
