package org.mesail.keycloak.sqlite.rebuild;

public record TableColumn(String name, String type, boolean notNull, String defaultValue, int primaryKeyPosition) {

    public TableColumn withNotNull(boolean value) {
        return new TableColumn(name, type, value, defaultValue, primaryKeyPosition);
    }

    public TableColumn withoutPrimaryKey() {
        return new TableColumn(name, type, notNull, defaultValue, 0);
    }

    public boolean isPrimaryKey() {
        return primaryKeyPosition > 0;
    }

    public boolean hasName(String other) {
        return name.equalsIgnoreCase(other);
    }
}
