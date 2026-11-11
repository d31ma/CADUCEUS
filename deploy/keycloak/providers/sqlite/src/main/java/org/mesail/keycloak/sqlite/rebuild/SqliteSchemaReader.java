package org.mesail.keycloak.sqlite.rebuild;

import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;

public class SqliteSchemaReader {

    private static final String COLUMNS_SQL = "SELECT name, type, \"notnull\", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid";
    private static final String INDEXES_SQL = "SELECT il.name, il.\"unique\", il.origin, m.sql FROM pragma_index_list(?) il "
            + "LEFT JOIN sqlite_master m ON m.type = 'index' AND m.name = il.name ORDER BY il.seq";
    private static final String INDEX_COLUMNS_SQL = "SELECT name FROM pragma_index_info(?) ORDER BY seqno";
    private static final String TRIGGERS_SQL = "SELECT sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = ? ORDER BY name";
    private static final int TRUE_FLAG = 1;

    private final Connection connection;

    public SqliteSchemaReader(Connection connection) {
        this.connection = connection;
    }

    public List<TableColumn> columns(String table) throws SQLException {
        return rows(COLUMNS_SQL, table, this::column);
    }

    public List<TableIndex> indexes(String table) throws SQLException {
        return rows(INDEXES_SQL, table, this::index);
    }

    public List<String> triggers(String table) throws SQLException {
        return rows(TRIGGERS_SQL, table, this::firstText);
    }

    private TableColumn column(ResultSet row) throws SQLException {
        return new TableColumn(row.getString(1), row.getString(2), row.getInt(3) == TRUE_FLAG, row.getString(4), row.getInt(5));
    }

    private TableIndex index(ResultSet row) throws SQLException {
        String name = row.getString(1);
        return new TableIndex(name, row.getString(4), row.getInt(2) == TRUE_FLAG, row.getString(3), rows(INDEX_COLUMNS_SQL, name, this::firstText));
    }

    private String firstText(ResultSet row) throws SQLException {
        return row.getString(1);
    }

    private <T> List<T> rows(String sql, String parameter, RowReader<T> reader) throws SQLException {
        List<T> values = new ArrayList<>();
        try (PreparedStatement statement = connection.prepareStatement(sql)) {
            statement.setString(1, parameter);
            try (ResultSet rows = statement.executeQuery()) {
                while (rows.next()) {
                    values.add(reader.read(rows));
                }
            }
        }
        return values;
    }
}
