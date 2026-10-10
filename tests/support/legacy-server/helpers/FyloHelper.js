import { Fylo } from '../shared/persistence/Fylo.js'

/**
 * Opens the FYLO database for one invocation. Tachyon builds providers per
 * request, so each request connects once, on first use, and every helper of
 * that request shares the connection.
 */
@Helper
export default class FyloHelper {
  /** @type {Promise<Fylo> | null} */
  #database = null

  /** @returns {Promise<Fylo>} */
  database() {
    this.#database ??= Fylo.createDb()
    return this.#database
  }
}
