
import { DomainMigrationRules } from '../shared/domains/DomainMigrationRules.js'

/** @import { StoredEmail } from '../types' */
/** @import { DomainMigration } from '../types' */
/** @import { DomainMigrationsHelper } from '../helpers/DomainMigrationsHelper.js' */
@Service
export default class DomainMigrationService {
  /** @type {DomainMigrationsHelper} */
  @Needs
  domainMigrationsHelper

  /**
   * @param {StoredEmail[]} emails
   * @returns {Promise<StoredEmail[]>}
   */
  async presentEmails(emails) {
    const migrations = await this.domainMigrationsHelper.list()
    if (migrations.length === 0) return emails
    return emails.map(email => DomainMigrationRules.rewriteForMigration(email, migrations))
  }

  /**
   * @param {StoredEmail} email
   * @returns {Promise<StoredEmail>}
   */
  async presentEmail(email) {
    const migrations = await this.domainMigrationsHelper.list()
    return DomainMigrationRules.rewriteForMigration(email, migrations)
  }
}

