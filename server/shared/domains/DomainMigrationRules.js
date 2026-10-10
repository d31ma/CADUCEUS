export class DomainMigrationRules {
  /**
   * @param {StoredEmail} email
   * @param {Array<DomainMigration & { docId?: string }>} migrations
   * @returns {StoredEmail}
   */
  static rewriteForMigration(email, migrations) {
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
}
