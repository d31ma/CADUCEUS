package org.mesail.keycloak.sqlite.liquibase;

import java.util.List;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.sql.Sql;
import liquibase.sqlgenerator.SqlGenerator;
import liquibase.sqlgenerator.SqlGeneratorChain;
import liquibase.sqlgenerator.core.AddColumnGenerator;
import liquibase.statement.core.AddColumnStatement;

public class SqliteAddColumnGenerator extends AddColumnGenerator {

    private static final int PRIORITY = SqlGenerator.PRIORITY_DATABASE + 1;

    @Override
    public int getPriority() {
        return PRIORITY;
    }

    @Override
    public boolean supports(AddColumnStatement statement, Database database) {
        return database instanceof SQLiteDatabase;
    }

    @SuppressWarnings("rawtypes")
    @Override
    public Sql[] generateSql(AddColumnStatement statement, Database database, SqlGeneratorChain chain) {
        return nativeOrRebuild(statement, database, chain);
    }

    @SuppressWarnings({"rawtypes", "unchecked"})
    private Sql[] nativeOrRebuild(AddColumnStatement statement, Database database, SqlGeneratorChain chain) {
        if (needsTableRebuild(statement)) {
            return chain.generateSql(statement, database);
        }
        return super.generateSql(statement, database, chain);
    }

    private List<AddColumnStatement> columnsOf(AddColumnStatement statement) {
        if (statement.isMultiple()) {
            return statement.getColumns();
        }
        return List.of(statement);
    }

    private boolean needsTableRebuild(AddColumnStatement statement) {
        return columnsOf(statement).stream().anyMatch(this::needsTableRebuildFor);
    }

    private boolean needsTableRebuildFor(AddColumnStatement column) {
        return isNotNullWithoutDefault(column) || column.isPrimaryKey();
    }

    private boolean isNotNullWithoutDefault(AddColumnStatement column) {
        return !column.isNullable() && column.getDefaultValue() == null;
    }
}
