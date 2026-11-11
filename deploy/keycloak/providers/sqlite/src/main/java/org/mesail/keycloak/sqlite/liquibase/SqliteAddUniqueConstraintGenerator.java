package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.AddUniqueConstraintStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteAddUniqueConstraintGenerator extends SqliteOnlyGenerator<AddUniqueConstraintStatement> {

    @Override
    public Sql[] generateSql(AddUniqueConstraintStatement statement, Database database, SqlGeneratorChain<AddUniqueConstraintStatement> chain) {
        return SqliteSql.createUniqueIndex(database, statement.getConstraintName(), statement.getTableName(), statement.getColumnNames());
    }
}
