import { Fylo } from '../shared/persistence/Fylo.js'
/** @import { DomainMigration } from '../types' */

@Helper
export default class DomainMigrationsHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @returns {Promise<Array<DomainMigration & { docId: string }>>}
   */
  async list() {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.DOMAIN_MIGRATIONS, { $ops: [] }).collect())
    return Object.entries(docs).map(([docId, migration]) => ({ docId, .../** @type {DomainMigration} */ (migration) }))
  }

  /**
   * @param {string} fromDomain
   * @param {string} toDomain
   * @returns {Promise<[string | null, DomainMigration | null]>}
   */
  async find(fromDomain, toDomain) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.DOMAIN_MIGRATIONS, {
      $ops: [{ fromDomain: { $eq: fromDomain } }, { toDomain: { $eq: toDomain } }],
    }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {DomainMigration} */ (entry[1])] : [null, null]
  }

  /**
   * @param {DomainMigration} migration
   * @returns {Promise<string>}
   */
  async put(migration) {
    const fylo = await this.fyloHelper.database()
    const [docId] = await this.find(migration.fromDomain, migration.toDomain)
    if (docId) {
      await fylo.patchDoc(Fylo.Collections.DOMAIN_MIGRATIONS, { [docId]: migration })
      return docId
    }
    return await fylo.putData(Fylo.Collections.DOMAIN_MIGRATIONS, migration)
  }
}
