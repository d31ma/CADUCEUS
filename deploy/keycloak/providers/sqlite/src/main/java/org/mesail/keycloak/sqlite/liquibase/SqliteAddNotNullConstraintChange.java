package org.mesail.keycloak.sqlite.liquibase;

import java.util.ArrayList;
import java.util.List;
import liquibase.change.ChangeMetaData;
import liquibase.change.DatabaseChange;
import liquibase.change.core.AddNotNullConstraintChange;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.statement.SqlStatement;
import liquibase.statement.core.RawParameterizedSqlStatement;
import org.mesail.keycloak.sqlite.rebuild.ColumnNullability;

@DatabaseChange(name = "addNotNullConstraint", description = "Makes a column NOT NULL by rebuilding the SQLite table",
        priority = ChangeMetaData.PRIORITY_DEFAULT + 1, appliesTo = "column")
public class SqliteAddNotNullConstraintChange extends AddNotNullConstraintChange {

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
        List<SqlStatement> statements = new ArrayList<>(fillNullValues());
        statements.addAll(List.of(SqliteChanges.rebuild(database, getTableName(), new ColumnNullability(getColumnName(), true))));
        return statements.toArray(new SqlStatement[0]);
    }

    private List<SqlStatement> fillNullValues() {
        if (getDefaultNullValue() == null) {
            return List.of();
        }
        String column = "\"" + getColumnName() + "\"";
        String value = "'" + getDefaultNullValue().replace("'", "''") + "'";
        return List.of(new RawParameterizedSqlStatement("UPDATE \"" + getTableName() + "\" SET " + column + " = " + value + " WHERE " + column + " IS NULL"));
    }
}
