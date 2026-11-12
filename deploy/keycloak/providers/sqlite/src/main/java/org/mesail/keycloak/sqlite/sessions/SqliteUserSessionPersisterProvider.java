package org.mesail.keycloak.sqlite.sessions;

import jakarta.persistence.EntityManager;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;
import org.keycloak.models.jpa.session.JpaUserSessionPersisterProvider;

public class SqliteUserSessionPersisterProvider extends JpaUserSessionPersisterProvider {

    private final KeycloakSession session;

    public SqliteUserSessionPersisterProvider(KeycloakSession session, EntityManager entityManager, int expirationBatch) {
        super(session, entityManager, expirationBatch);
        this.session = session;
    }

    @Override
    public void removeExpired(RealmModel realm) {
        session.getTransactionManager().enlistAfterCompletion(new ExpiredSessionRemoval(this, realm));
    }

    void removeExpiredNow(RealmModel realm) {
        super.removeExpired(realm);
    }
}
