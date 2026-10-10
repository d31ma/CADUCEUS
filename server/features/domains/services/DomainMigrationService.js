
import { DomainMigrationRules } from '../../../shared/domains/DomainMigrationRules.js'

/** @import { StoredEmail } from '../../../types' */
/** @import { DomainMigration } from '../../../types' */
/** @import { DomainMigrationsRepository } from '../repositories/DomainMigrationsRepository.js' */
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
    return emails.map(email => DomainMigrationRules.rewriteForMigration(email, migrations))
  }

  /**
   * @param {StoredEmail} email
   * @returns {Promise<StoredEmail>}
   */
  async presentEmail(email) {
    const migrations = await this.domainMigrations.list()
    return DomainMigrationRules.rewriteForMigration(email, migrations)
  }
}

