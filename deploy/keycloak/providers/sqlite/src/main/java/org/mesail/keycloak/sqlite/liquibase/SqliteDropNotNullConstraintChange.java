package org.mesail.keycloak.sqlite.liquibase;

import liquibase.change.ChangeMetaData;
import liquibase.change.DatabaseChange;
import liquibase.change.core.DropNotNullConstraintChange;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.statement.SqlStatement;
import org.mesail.keycloak.sqlite.rebuild.ColumnNullability;

@DatabaseChange(name = "dropNotNullConstraint", description = "Makes a column nullable by rebuilding the SQLite table",
        priority = ChangeMetaData.PRIORITY_DEFAULT + 1, appliesTo = "column")
public class SqliteDropNotNullConstraintChange extends DropNotNullConstraintChange {

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
        return SqliteChanges.rebuild(database, getTableName(), new ColumnNullability(getColumnName(), false));
    }
}
