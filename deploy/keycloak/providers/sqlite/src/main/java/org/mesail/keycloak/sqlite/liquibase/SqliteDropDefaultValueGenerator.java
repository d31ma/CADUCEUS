package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.DropDefaultValueStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteDropDefaultValueGenerator extends SqliteOnlyGenerator<DropDefaultValueStatement> {

    @Override
    public Sql[] generateSql(DropDefaultValueStatement statement, Database database, SqlGeneratorChain<DropDefaultValueStatement> chain) {
        return SqliteSql.none();
    }
}
