import { Fylo } from './Fylo.js'

/**
 * Compatibility facade for integrations written against the pre-container
 * repository API.
 */
export class DatabaseRepository {
  /** @param {string} [root] */
  static createDb(root) {
    return Fylo.createDb(root)
  }
}
