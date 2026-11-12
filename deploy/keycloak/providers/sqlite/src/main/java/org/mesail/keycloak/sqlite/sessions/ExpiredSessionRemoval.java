package org.mesail.keycloak.sqlite.sessions;

import org.keycloak.models.AbstractKeycloakTransaction;
import org.keycloak.models.RealmModel;

public class ExpiredSessionRemoval extends AbstractKeycloakTransaction {

    private final SqliteUserSessionPersisterProvider persister;
    private final RealmModel realm;

    public ExpiredSessionRemoval(SqliteUserSessionPersisterProvider persister, RealmModel realm) {
        this.persister = persister;
        this.realm = realm;
    }

    @Override
    protected void commitImpl() {
        persister.removeExpiredNow(realm);
    }

    @Override
    protected void rollbackImpl() {
    }
}
