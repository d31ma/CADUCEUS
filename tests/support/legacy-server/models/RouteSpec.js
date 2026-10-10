/**
 * @typedef {object} RouteSpec
 * @property {Record<string, string>} [paths] - The `@FromPath` values of the route.
 * @property {AuthOptions} [guard] - Ask the Go authorization boundary before the body runs.
 * @property {string} [signature] - Require an `X-Caduceus-Signature` HMAC made with this setting's secret.
 * @property {boolean} [testOnly] - Answer only while test routes are enabled.
 */
export {}
