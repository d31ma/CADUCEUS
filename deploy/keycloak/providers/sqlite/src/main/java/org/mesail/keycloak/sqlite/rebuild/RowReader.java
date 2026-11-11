package org.mesail.keycloak.sqlite.rebuild;

import java.sql.ResultSet;
import java.sql.SQLException;

public interface RowReader<T> {

    T read(ResultSet row) throws SQLException;
}
