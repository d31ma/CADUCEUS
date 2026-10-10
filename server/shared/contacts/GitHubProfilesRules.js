import { Security } from '../security/Security.js'

export class GitHubProfilesRules {
  static API_ROOT = 'https://api.github.com'

  static MAX_RESPONSE_BYTES = 1_000_000

  static MAX_CACHE_ENTRIES = 256

  static FOUND_TTL_MS = 60 * 60 * 1000

  static NOT_FOUND_TTL_MS = 10 * 60 * 1000

  static REQUEST_TIMEOUT_MS = 5_000

  static LOGIN_PATTERN = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/

  static REPOSITORY_PATTERN = /^[A-Za-z0-9_.-]{1,100}$/

  /** @param {unknown} value */
  static cleanLogin(value) {
    const login = typeof value === 'string' ? value.trim() : ''
    return GitHubProfilesRules.LOGIN_PATTERN.test(login) ? login : ''
  }

  /** @param {unknown} value @param {number} max */
  static cleanText(value, max) {
    if (typeof value !== 'string') return ''
    const text = value.replace(/[\0-\x1f\x7f]/g, ' ').replace(/\s+/g, ' ').trim()
    return text.slice(0, max)
  }

  /** @param {any} repository @param {string} login */
  static sanitizeRepository(repository, login) {
    const name = GitHubProfilesRules.cleanText(repository?.name, 100)
    const owner = GitHubProfilesRules.cleanLogin(repository?.owner?.login)
    if (!GitHubProfilesRules.REPOSITORY_PATTERN.test(name) || !owner || owner.toLowerCase() !== login.toLowerCase()) return []
    const stars = Number.isSafeInteger(repository?.stargazers_count) && repository.stargazers_count >= 0
      ? Math.min(repository.stargazers_count, 2_147_483_647)
      : 0
    const fullName = `${owner}/${name}`
    return [{
      name,
      fullName,
      url: `https://github.com/${owner}/${name}`,
      description: GitHubProfilesRules.cleanText(repository?.description, 300),
      language: GitHubProfilesRules.cleanText(repository?.language, 50),
      stars,
    }]
  }
}
