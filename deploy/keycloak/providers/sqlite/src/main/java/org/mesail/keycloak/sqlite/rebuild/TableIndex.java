package org.mesail.keycloak.sqlite.rebuild;

import java.util.List;

public record TableIndex(String name, String sql, boolean unique, String origin, List<String> columns) {

    private static final String PRIMARY_KEY_ORIGIN = "pk";
    private static final String UNIQUE_CONSTRAINT_ORIGIN = "u";

    public boolean belongsToPrimaryKey() {
        return PRIMARY_KEY_ORIGIN.equals(origin);
    }

    public boolean isInlineUniqueConstraint() {
        return UNIQUE_CONSTRAINT_ORIGIN.equals(origin);
    }

    public boolean hasName(String other) {
        return name.equalsIgnoreCase(other);
    }

    public boolean usesColumn(String column) {
        return columns.stream().anyMatch(column::equalsIgnoreCase);
    }
}
