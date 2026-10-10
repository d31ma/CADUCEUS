/**
 * Fylo stores nested arrays as JSON strings. These helpers convert between
 * the typed DomainConfig interface and the flat shape stored on disk.
 * @typedef {Omit<import('../../../types').DomainConfig, 'routes'> & { routes: string | import('../../../types').RouteRule[] }} RawDomainDoc
 * @typedef {import('../../../types').DomainConfig} DomainConfig
 * @typedef {import('../../../types').RouteRule} RouteRule
 */
export {}
