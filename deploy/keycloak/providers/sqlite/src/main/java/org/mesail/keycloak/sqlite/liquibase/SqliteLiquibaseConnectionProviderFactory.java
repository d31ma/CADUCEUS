package org.mesail.keycloak.sqlite.liquibase;

import java.sql.Connection;
import liquibase.database.Database;
import liquibase.database.core.SQLiteDatabase;
import liquibase.database.jvm.JdbcConnection;
import liquibase.resource.ClassLoaderResourceAccessor;
import org.keycloak.Config;
import org.keycloak.connections.jpa.updater.liquibase.LiquibaseJpaUpdaterProvider;
import org.keycloak.connections.jpa.updater.liquibase.conn.DefaultLiquibaseConnectionProvider;
import org.keycloak.connections.jpa.updater.liquibase.conn.KeycloakLiquibase;
import org.mesail.keycloak.sqlite.metadata.SqliteMetadataConnection;

public class SqliteLiquibaseConnectionProviderFactory extends DefaultLiquibaseConnectionProvider {

    public static final String PROVIDER_ID = "sqlite";
    private static final String INDEX_THRESHOLD_KEY = "indexCreationThreshold";
    private static final long DEFAULT_INDEX_THRESHOLD = 300_000L;

    private long indexCreationThreshold = DEFAULT_INDEX_THRESHOLD;

    @Override
    public void init(Config.Scope config) {
        super.init(config);
        indexCreationThreshold = config.getLong(INDEX_THRESHOLD_KEY, DEFAULT_INDEX_THRESHOLD);
    }

    @Override
    public String getId() {
        return PROVIDER_ID;
    }

    @Override
    public KeycloakLiquibase getLiquibase(Connection connection, String defaultSchema) {
        Database database = createDatabase(connection);
        ClassLoaderResourceAccessor resources = new ClassLoaderResourceAccessor(getClass().getClassLoader());
        return new KeycloakLiquibase(LiquibaseJpaUpdaterProvider.CHANGELOG, resources, database);
    }

    @Override
    public KeycloakLiquibase getLiquibaseForCustomUpdate(Connection connection, String defaultSchema, String changelogLocation,
            ClassLoader classLoader, String changelogTableName) {
        Database database = createDatabase(connection);
        database.setDatabaseChangeLogTableName(changelogTableName);
        return new KeycloakLiquibase(changelogLocation, new ClassLoaderResourceAccessor(classLoader), database);
    }

    private Database createDatabase(Connection connection) {
        SQLiteDatabase database = new SQLiteDatabase();
        database.setConnection(new JdbcConnection(SqliteMetadataConnection.wrap(connection)));
        database.set(INDEX_CREATION_THRESHOLD_PARAM, indexCreationThreshold);
        return database;
    }
}
