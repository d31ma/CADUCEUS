package org.mesail.keycloak.sqlite.rebuild;

import java.sql.Connection;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import liquibase.statement.SqlStatement;
import liquibase.statement.core.RawParameterizedSqlStatement;

public class SqliteTableRebuilder {

    private static final String REBUILD_SUFFIX = "__rebuild";
    private static final String UNIQUE_NAME_PART = "__unique_";
    private static final String SEPARATOR = ", ";

    private final SqliteSchemaReader reader;

    public SqliteTableRebuilder(Connection connection) {
        this.reader = new SqliteSchemaReader(connection);
    }

    public SqlStatement[] statements(String table, TableRebuild rebuild) throws SQLException {
        List<TableColumn> oldColumns = reader.columns(table);
        List<TableColumn> newColumns = newColumns(oldColumns, rebuild);
        List<String> sql = new ArrayList<>();
        sql.add("PRAGMA legacy_alter_table = ON");
        sql.add(createTable(table + REBUILD_SUFFIX, newColumns));
        sql.add(copyRows(table, newColumns));
        sql.add("DROP TABLE " + quote(table));
        sql.add("ALTER TABLE " + quote(table + REBUILD_SUFFIX) + " RENAME TO " + quote(table));
        sql.addAll(indexStatements(table, reader.indexes(table), rebuild));
        sql.addAll(reader.triggers(table));
        sql.add("PRAGMA legacy_alter_table = OFF");
        return sql.stream().map(RawParameterizedSqlStatement::new).toArray(SqlStatement[]::new);
    }

    private List<TableColumn> newColumns(List<TableColumn> oldColumns, TableRebuild rebuild) {
        return oldColumns.stream().filter(rebuild::keepsColumn).map(rebuild::adjust).collect(Collectors.toList());
    }

    private String createTable(String table, List<TableColumn> columns) {
        List<String> parts = columns.stream().map(this::columnDefinition).collect(Collectors.toList());
        parts.addAll(primaryKeyClause(columns));
        return "CREATE TABLE " + quote(table) + " (" + String.join(SEPARATOR, parts) + ")";
    }

    private String columnDefinition(TableColumn column) {
        return quote(column.name()) + typeClause(column) + notNullClause(column) + defaultClause(column);
    }

    private String typeClause(TableColumn column) {
        if (column.type() == null || column.type().isBlank()) {
            return "";
        }
        return " " + column.type();
    }

    private String notNullClause(TableColumn column) {
        if (column.notNull()) {
            return " NOT NULL";
        }
        return "";
    }

    private String defaultClause(TableColumn column) {
        if (column.defaultValue() == null) {
            return "";
        }
        return " DEFAULT " + column.defaultValue();
    }

    private List<String> primaryKeyClause(List<TableColumn> columns) {
        List<String> keyColumns = columns.stream().filter(TableColumn::isPrimaryKey)
                .sorted((left, right) -> Integer.compare(left.primaryKeyPosition(), right.primaryKeyPosition()))
                .map(column -> quote(column.name())).collect(Collectors.toList());
        if (keyColumns.isEmpty()) {
            return List.of();
        }
        return List.of("PRIMARY KEY (" + String.join(SEPARATOR, keyColumns) + ")");
    }

    private String copyRows(String table, List<TableColumn> columns) {
        String names = columns.stream().map(column -> quote(column.name())).collect(Collectors.joining(SEPARATOR));
        return "INSERT INTO " + quote(table + REBUILD_SUFFIX) + " (" + names + ") SELECT " + names + " FROM " + quote(table);
    }

    private List<String> indexStatements(String table, List<TableIndex> indexes, TableRebuild rebuild) {
        return IntStream.range(0, indexes.size()).filter(position -> keepsIndex(indexes.get(position), rebuild))
                .mapToObj(position -> indexDefinition(table, indexes.get(position), position)).collect(Collectors.toList());
    }

    private boolean keepsIndex(TableIndex index, TableRebuild rebuild) {
        return !index.belongsToPrimaryKey() && rebuild.keepsIndex(index);
    }

    private String indexDefinition(String table, TableIndex index, int position) {
        if (index.isInlineUniqueConstraint()) {
            return uniqueIndexDefinition(table, index, position);
        }
        return index.sql();
    }

    private String uniqueIndexDefinition(String table, TableIndex index, int position) {
        String columns = index.columns().stream().map(this::quote).collect(Collectors.joining(SEPARATOR));
        return "CREATE UNIQUE INDEX " + quote(table + UNIQUE_NAME_PART + position) + " ON " + quote(table) + " (" + columns + ")";
    }

    private String quote(String name) {
        return "\"" + name.replace("\"", "\"\"") + "\"";
    }
}
