package org.mesail.keycloak.sqlite.sessions;

import jakarta.persistence.EntityManager;
import org.keycloak.Config;
import org.keycloak.connections.jpa.JpaConnectionProvider;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.jpa.session.JpaUserSessionPersisterProviderFactory;
import org.keycloak.models.session.UserSessionPersisterProvider;

public class SqliteUserSessionPersisterProviderFactory extends JpaUserSessionPersisterProviderFactory {

    public static final String PROVIDER_ID = "sqlite";
    private static final String BATCH_SETTING = "expirationBatch";
    private static final int DEFAULT_BATCH = 512;
    private static final int SMALLEST_BATCH = 1;

    private int expirationBatch = DEFAULT_BATCH;

    @Override
    public void init(Config.Scope config) {
        super.init(config);
        expirationBatch = Math.max(SMALLEST_BATCH, config.getInt(BATCH_SETTING, DEFAULT_BATCH));
    }

    @Override
    public UserSessionPersisterProvider create(KeycloakSession session) {
        EntityManager entityManager = session.getProvider(JpaConnectionProvider.class).getEntityManager();
        return new SqliteUserSessionPersisterProvider(session, entityManager, expirationBatch);
    }

    @Override
    public String getId() {
        return PROVIDER_ID;
    }
}
