package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.DropForeignKeyConstraintStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteDropForeignKeyGenerator extends SqliteOnlyGenerator<DropForeignKeyConstraintStatement> {

    @Override
    public Sql[] generateSql(DropForeignKeyConstraintStatement statement, Database database, SqlGeneratorChain<DropForeignKeyConstraintStatement> chain) {
        return SqliteSql.dropTrigger(database, statement.getConstraintName());
    }
}
