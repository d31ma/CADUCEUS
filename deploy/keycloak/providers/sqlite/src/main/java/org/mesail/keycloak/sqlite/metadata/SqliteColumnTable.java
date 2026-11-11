package org.mesail.keycloak.sqlite.metadata;

import java.sql.Connection;
import java.sql.DatabaseMetaData;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.ResultSetMetaData;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public class SqliteColumnTable {

    private static final String TEMP_TABLE = "temp.mesail_metadata_columns";
    private static final String MATCH_ALL = "%";
    private static final String MISSING_TABLE = "mesail_metadata_no_such_table";
    private static final String TABLE_NAME_LABEL = "TABLE_NAME";
    private static final String[] TABLE_TYPES = {"TABLE"};
    private static final String SEPARATOR = ", ";

    private final DatabaseMetaData metadata;
    private final Connection connection;

    public SqliteColumnTable(DatabaseMetaData metadata, Connection connection) {
        this.metadata = metadata;
        this.connection = connection;
    }

    public ResultSet columnsOfAllTables(String catalog, String schema, String columnPattern) throws SQLException {
        execute("DROP TABLE IF EXISTS " + TEMP_TABLE);
        createTempTable(catalog, schema);
        copyAllTables(catalog, schema, columnPattern);
        Statement statement = connection.createStatement();
        statement.closeOnCompletion();
        return statement.executeQuery("SELECT * FROM " + TEMP_TABLE + " ORDER BY rowid");
    }

    private void createTempTable(String catalog, String schema) throws SQLException {
        try (ResultSet columns = metadata.getColumns(catalog, schema, MISSING_TABLE, MATCH_ALL)) {
            List<String> labels = labels(columns.getMetaData());
            execute("CREATE TEMP TABLE mesail_metadata_columns (" + String.join(SEPARATOR, labels) + ")");
        }
    }

    private void copyAllTables(String catalog, String schema, String columnPattern) throws SQLException {
        for (String table : tableNames(catalog, schema)) {
            copyColumns(catalog, schema, table, columnPattern);
        }
    }

    private List<String> tableNames(String catalog, String schema) throws SQLException {
        List<String> names = new ArrayList<>();
        try (ResultSet tables = metadata.getTables(catalog, schema, MATCH_ALL, TABLE_TYPES)) {
            while (tables.next()) {
                names.add(tables.getString(TABLE_NAME_LABEL));
            }
        }
        return names;
    }

    private void copyColumns(String catalog, String schema, String table, String columnPattern) throws SQLException {
        try (ResultSet columns = metadata.getColumns(catalog, schema, table, columnPattern);
                PreparedStatement insert = prepareInsert(columns.getMetaData().getColumnCount())) {
            while (columns.next()) {
                insertRowOfTable(insert, columns, table);
            }
        }
    }

    private PreparedStatement prepareInsert(int width) throws SQLException {
        String placeholders = String.join(SEPARATOR, Collections.nCopies(width, "?"));
        return connection.prepareStatement("INSERT INTO " + TEMP_TABLE + " VALUES (" + placeholders + ")");
    }

    private void insertRowOfTable(PreparedStatement insert, ResultSet columns, String table) throws SQLException {
        if (!table.equals(columns.getString(TABLE_NAME_LABEL))) {
            return;
        }
        copyValues(insert, columns);
        insert.executeUpdate();
    }

    private void copyValues(PreparedStatement insert, ResultSet columns) throws SQLException {
        for (int position = 1; position <= columns.getMetaData().getColumnCount(); position++) {
            insert.setObject(position, columns.getObject(position));
        }
    }

    private List<String> labels(ResultSetMetaData columnsMetadata) throws SQLException {
        List<String> labels = new ArrayList<>();
        for (int position = 1; position <= columnsMetadata.getColumnCount(); position++) {
            labels.add("\"" + columnsMetadata.getColumnLabel(position) + "\"");
        }
        return labels;
    }

    private void execute(String sql) throws SQLException {
        try (Statement statement = connection.createStatement()) {
            statement.execute(sql);
        }
    }
}
