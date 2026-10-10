import { Collections, collect } from '../../../platform/persistence/fylo.js'

export class DomainMigrationsRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @returns {Promise<Array<DomainMigration & { docId: string }>>}
   */
  async list() {
    const docs = await collect(this.fylo.findDocs(Collections.DOMAIN_MIGRATIONS, { $ops: [] }).collect())
    return Object.entries(docs).map(([docId, migration]) => ({ docId, .../** @type {DomainMigration} */ (migration) }))
  }

  /**
   * @param {string} fromDomain
   * @param {string} toDomain
   * @returns {Promise<[string | null, DomainMigration | null]>}
   */
  async find(fromDomain, toDomain) {
    const docs = await collect(this.fylo.findDocs(Collections.DOMAIN_MIGRATIONS, {
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
    const [docId] = await this.find(migration.fromDomain, migration.toDomain)
    if (docId) {
      await this.fylo.patchDoc(Collections.DOMAIN_MIGRATIONS, { [docId]: migration })
      return docId
    }
    return await this.fylo.putData(Collections.DOMAIN_MIGRATIONS, migration)
  }
}
