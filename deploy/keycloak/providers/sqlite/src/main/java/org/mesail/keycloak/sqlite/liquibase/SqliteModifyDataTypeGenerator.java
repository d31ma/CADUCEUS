package org.mesail.keycloak.sqlite.liquibase;

import liquibase.database.Database;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.statement.core.ModifyDataTypeStatement;
import org.mesail.keycloak.sqlite.utils.SqliteSql;

public class SqliteModifyDataTypeGenerator extends SqliteOnlyGenerator<ModifyDataTypeStatement> {

    @Override
    public Sql[] generateSql(ModifyDataTypeStatement statement, Database database, SqlGeneratorChain<ModifyDataTypeStatement> chain) {
        return SqliteSql.none();
    }
}
