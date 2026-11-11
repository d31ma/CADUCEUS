package org.mesail.keycloak.sqlite.rebuild;

public class PrimaryKeyRemoval implements TableRebuild {

    private final String primaryKeyIndexName;

    public PrimaryKeyRemoval(String primaryKeyIndexName) {
        this.primaryKeyIndexName = primaryKeyIndexName;
    }

    @Override
    public boolean keepsColumn(TableColumn column) {
        return true;
    }

    @Override
    public TableColumn adjust(TableColumn column) {
        return column.withoutPrimaryKey();
    }

    @Override
    public boolean keepsIndex(TableIndex index) {
        return !index.hasName(primaryKeyIndexName);
    }
}
