package org.mesail.keycloak.sqlite.utils;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sql.UnparsedSql;
import liquibase.structure.core.Index;
import liquibase.structure.core.Table;

public final class SqliteSql {

    private static final Sql[] NO_SQL = new Sql[0];

    private SqliteSql() {
    }

    public static Sql[] none() {
        return NO_SQL;
    }

    public static Sql[] single(String sql) {
        return new Sql[] {new UnparsedSql(sql)};
    }

    public static Sql[] createUniqueIndex(Database database, String indexName, String tableName, String columnNames) {
        String sql = "CREATE UNIQUE INDEX IF NOT EXISTS " + database.escapeObjectName(indexName, Index.class)
                + " ON " + database.escapeObjectName(tableName, Table.class)
                + " (" + database.escapeColumnNameList(columnNames) + ")";
        return single(sql);
    }

    public static Sql[] dropIndex(Database database, String indexName) {
        return single("DROP INDEX IF EXISTS " + database.escapeObjectName(indexName, Index.class));
    }

    public static Sql[] dropTrigger(Database database, String triggerName) {
        return single("DROP TRIGGER IF EXISTS " + database.escapeObjectName(triggerName, Table.class));
    }
}
