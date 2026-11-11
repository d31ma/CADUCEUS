package org.mesail.keycloak.sqlite.liquibase;

import liquibase.change.ChangeMetaData;
import liquibase.change.DatabaseChange;
import liquibase.change.core.DropPrimaryKeyChange;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.statement.SqlStatement;
import org.mesail.keycloak.sqlite.rebuild.PrimaryKeyRemoval;

@DatabaseChange(name = "dropPrimaryKey", description = "Drops a primary key by rebuilding the SQLite table",
        priority = ChangeMetaData.PRIORITY_DEFAULT + 1, appliesTo = "primaryKey")
public class SqliteDropPrimaryKeyChange extends DropPrimaryKeyChange {

    @Override
    public boolean supports(Database database) {
        return database instanceof SQLiteDatabase;
    }

    @Override
    public boolean generateStatementsVolatile(Database database) {
        return true;
    }

    @Override
    public SqlStatement[] generateStatements(Database database) {
        return SqliteChanges.rebuild(database, getTableName(), new PrimaryKeyRemoval(String.valueOf(getConstraintName())));
    }
}
