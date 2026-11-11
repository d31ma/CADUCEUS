package org.mesail.keycloak.sqlite.rebuild;

import java.util.List;

public class DroppedColumns implements TableRebuild {

    private final List<String> names;

    public DroppedColumns(List<String> names) {
        this.names = names;
    }

    @Override
    public boolean keepsColumn(TableColumn column) {
        return names.stream().noneMatch(column::hasName);
    }

    @Override
    public TableColumn adjust(TableColumn column) {
        return column;
    }

    @Override
    public boolean keepsIndex(TableIndex index) {
        return names.stream().noneMatch(index::usesColumn);
    }
}
