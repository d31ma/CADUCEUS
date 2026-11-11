package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.DropUniqueConstraintStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteDropUniqueConstraintGenerator extends SqliteOnlyGenerator<DropUniqueConstraintStatement> {

    @Override
    public Sql[] generateSql(DropUniqueConstraintStatement statement, Database database, SqlGeneratorChain<DropUniqueConstraintStatement> chain) {
        return SqliteSql.dropIndex(database, statement.getConstraintName());
    }
}
