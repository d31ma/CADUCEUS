package org.mesail.keycloak.sqlite.rebuild;

public class ColumnNullability implements TableRebuild {

    private final String columnName;
    private final boolean notNull;

    public ColumnNullability(String columnName, boolean notNull) {
        this.columnName = columnName;
        this.notNull = notNull;
    }

    @Override
    public boolean keepsColumn(TableColumn column) {
        return true;
    }

    @Override
    public TableColumn adjust(TableColumn column) {
        return adjustNamedColumn(column);
    }

    @Override
    public boolean keepsIndex(TableIndex index) {
        return true;
    }

    private TableColumn adjustNamedColumn(TableColumn column) {
        if (column.hasName(columnName)) {
            return column.withNotNull(notNull);
        }
        return column;
    }
}
