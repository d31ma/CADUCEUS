/** @typedef {import('../../../types').StoredEmail} StoredEmail */
/** @typedef {import('../../../types').DomainMigration} DomainMigration */
/** @typedef {import('../repositories/domain-migrations.js').DomainMigrationsRepository} DomainMigrationsRepository */

export class DomainMigrationService {
  /** @param {DomainMigrationsRepository} domainMigrations */
  constructor(domainMigrations) {
    this.domainMigrations = domainMigrations
  }

  /**
   * @param {StoredEmail[]} emails
   * @returns {Promise<StoredEmail[]>}
   */
  async presentEmails(emails) {
    const migrations = await this.domainMigrations.list()
    if (migrations.length === 0) return emails
    return emails.map(email => rewriteForMigration(email, migrations))
  }

  /**
   * @param {StoredEmail} email
   * @returns {Promise<StoredEmail>}
   */
  async presentEmail(email) {
    const migrations = await this.domainMigrations.list()
    return rewriteForMigration(email, migrations)
  }
}

/**
 * @param {StoredEmail} email
 * @param {Array<DomainMigration & { docId?: string }>} migrations
 * @returns {StoredEmail}
 */
function rewriteForMigration(email, migrations) {
  for (const migration of migrations) {
    const fromSuffix = `@${migration.fromDomain}`
    const toSuffix = `@${migration.toDomain}`
    if (email.recipient.endsWith(fromSuffix)) {
      return {
        ...email,
        recipient: email.recipient.replace(new RegExp(`${fromSuffix}$`), toSuffix),
        originalRecipient: email.originalRecipient || email.recipient,
      }
    }
  }
  return email
}
