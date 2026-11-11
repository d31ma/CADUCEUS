package org.mesail.keycloak.sqlite.rebuild;

public interface TableRebuild {

    boolean keepsColumn(TableColumn column);

    TableColumn adjust(TableColumn column);

    boolean keepsIndex(TableIndex index);
}
