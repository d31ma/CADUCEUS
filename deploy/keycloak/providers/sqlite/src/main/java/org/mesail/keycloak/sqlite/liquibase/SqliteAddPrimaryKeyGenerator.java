package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.AddPrimaryKeyStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteAddPrimaryKeyGenerator extends SqliteOnlyGenerator<AddPrimaryKeyStatement> {

    private static final String NAME_PREFIX = "PK_";

    @Override
    public Sql[] generateSql(AddPrimaryKeyStatement statement, Database database, SqlGeneratorChain<AddPrimaryKeyStatement> chain) {
        String indexName = indexName(statement);
        return SqliteSql.createUniqueIndex(database, indexName, statement.getTableName(), statement.getColumnNames());
    }

    private String indexName(AddPrimaryKeyStatement statement) {
        if (statement.getConstraintName() != null) {
            return statement.getConstraintName();
        }
        return NAME_PREFIX + statement.getTableName();
    }
}
