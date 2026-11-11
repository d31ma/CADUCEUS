package org.mesail.keycloak.sqlite.liquibase;

import java.util.List;
import java.util.stream.Collectors;
import liquibase.change.ChangeMetaData;
import liquibase.change.ColumnConfig;
import liquibase.change.DatabaseChange;
import liquibase.change.core.DropColumnChange;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.statement.SqlStatement;
import org.mesail.keycloak.sqlite.rebuild.DroppedColumns;

@DatabaseChange(name = "dropColumn", description = "Drops columns by rebuilding the SQLite table",
        priority = ChangeMetaData.PRIORITY_DEFAULT + 1, appliesTo = "column")
public class SqliteDropColumnChange extends DropColumnChange {

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
        return SqliteChanges.rebuild(database, getTableName(), new DroppedColumns(columnNames()));
    }

    private List<String> columnNames() {
        if (getColumnName() != null) {
            return List.of(getColumnName());
        }
        return getColumns().stream().map(ColumnConfig::getName).collect(Collectors.toList());
    }
}
