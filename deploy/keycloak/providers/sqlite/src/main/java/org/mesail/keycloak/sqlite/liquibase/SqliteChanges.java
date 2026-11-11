package org.mesail.keycloak.sqlite.liquibase;

import java.sql.Connection;
import java.sql.SQLException;
import liquibase.database.Database;
import liquibase.database.jvm.JdbcConnection;
import liquibase.exception.UnexpectedLiquibaseException;
import liquibase.statement.SqlStatement;
import org.mesail.keycloak.sqlite.rebuild.SqliteTableRebuilder;
import org.mesail.keycloak.sqlite.rebuild.TableRebuild;

public final class SqliteChanges {

    private SqliteChanges() {
    }

    public static SqlStatement[] rebuild(Database database, String table, TableRebuild rebuild) {
        Connection connection = ((JdbcConnection) database.getConnection()).getUnderlyingConnection();
        return rebuildStatements(connection, table, rebuild);
    }

    private static SqlStatement[] rebuildStatements(Connection connection, String table, TableRebuild rebuild) {
        try {
            return new SqliteTableRebuilder(connection).statements(table, rebuild);
        } catch (SQLException failure) {
            throw new UnexpectedLiquibaseException("Could not read the SQLite schema of table " + table, failure);
        }
    }
}
