import { Security } from '../../../shared/security/Security.js'
import { GitHubProfilesRules } from '../../../shared/contacts/GitHubProfilesRules.js'

/**
 * Optional, privacy-gated GitHub contact enrichment.
 *
 * The caller decides whether a mailbox has opted in before invoking this
 * service. This class then talks only to the fixed public GitHub API origin,
 * accepts a small allowlisted response shape, and derives every returned URL
 * locally. No caller-controlled URL or redirect is followed.
 */
export class GitHubProfilesService {
  /**
   * @param {{ fetchImpl?: typeof fetch, now?: () => number, timeoutMs?: number }} [options]
   */
  constructor(options = {}) {
    this.fetchImpl = options.fetchImpl ?? globalThis.fetch
    this.now = options.now ?? Date.now
    this.timeoutMs = options.timeoutMs ?? GitHubProfilesRules.REQUEST_TIMEOUT_MS
    /** @type {Map<string, { expiresAt: number, value: GitHubLookupResult }>} */
    this.cache = new Map()
  }

  /**
   * Find the first public GitHub user matching an exact normalized email, then
   * return at most five public repositories ordered by star count.
   *
   * @param {string} email
   * @returns {Promise<GitHubLookupResult>}
   */
  async findByEmail(email) {
    const address = Security.normalizeEmailAddress(email)
    if (!address) return { status: 'not-found' }

    const cached = this.cache.get(address)
    if (cached && cached.expiresAt > this.now()) return cached.value
    if (cached) this.cache.delete(address)

    try {
      const users = await this.#request(`/search/users?q=${encodeURIComponent(address)}&per_page=1`)
      const login = GitHubProfilesRules.cleanLogin(users?.items?.[0]?.login)
      if (!login) return this.#remember(address, { status: 'not-found' }, GitHubProfilesRules.NOT_FOUND_TTL_MS)

      const repositories = await this.#request(
        `/search/repositories?q=${encodeURIComponent(`user:${login}`)}&sort=stars&order=desc&per_page=5`,
      )
      const profile = {
        login,
        profileUrl: `https://github.com/${login}`,
        repositories: Array.isArray(repositories?.items)
          ? repositories.items.flatMap(repository => GitHubProfilesRules.sanitizeRepository(repository, login)).slice(0, 5)
          : [],
      }
      profile.repositories.sort((a, b) => b.stars - a.stars || a.fullName.localeCompare(b.fullName))
      return this.#remember(address, { status: 'found', profile }, GitHubProfilesRules.FOUND_TTL_MS)
    } catch {
      // GitHub enrichment is optional. A network, quota, or response failure
      // must never interfere with local contact and message context.
      return { status: 'unavailable' }
    }
  }

  /** @param {string} path */
  async #request(path) {
    if (typeof this.fetchImpl !== 'function') throw new Error('fetch unavailable')
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeoutMs)
    try {
      const response = await this.fetchImpl(`${GitHubProfilesRules.API_ROOT}${path}`, {
        headers: {
          Accept: 'application/vnd.github+json',
          'User-Agent': 'CADUCEUS-Mail-Server',
          'X-GitHub-Api-Version': '2022-11-28',
        },
        redirect: 'error',
        signal: controller.signal,
      })
      if (!response?.ok) throw new Error(`GitHub answered ${response?.status || 0}`)
      const declaredLength = Number(response.headers?.get?.('content-length') || 0)
      if (declaredLength > GitHubProfilesRules.MAX_RESPONSE_BYTES) throw new Error('GitHub response is too large')
      const text = await response.text()
      if (new TextEncoder().encode(text).byteLength > GitHubProfilesRules.MAX_RESPONSE_BYTES) {
        throw new Error('GitHub response is too large')
      }
      const parsed = JSON.parse(text)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('Invalid GitHub response')
      return parsed
    } finally {
      clearTimeout(timer)
    }
  }

  /** @param {string} address @param {GitHubLookupResult} value @param {number} ttl */
  #remember(address, value, ttl) {
    if (this.cache.size >= GitHubProfilesRules.MAX_CACHE_ENTRIES) {
      const oldest = this.cache.keys().next().value
      if (oldest) this.cache.delete(oldest)
    }
    this.cache.set(address, { expiresAt: this.now() + ttl, value })
    return value
  }
}

