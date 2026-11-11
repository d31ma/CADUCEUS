package org.mesail.keycloak.sqlite.lock;

import java.util.EnumMap;
import java.util.Map;
import org.keycloak.Config;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.KeycloakSessionFactory;
import org.keycloak.models.dblock.DBLockProvider;
import org.keycloak.models.dblock.DBLockProviderFactory;

public class SqliteLockProviderFactory implements DBLockProviderFactory {

    public static final String PROVIDER_ID = "sqlite";
    private static final long DEFAULT_WAIT_MILLIS = 900_000L;
    private static final int PROVIDER_ORDER = 0;

    private final Map<DBLockProvider.Namespace, NamespaceLock> locks = createLocks();
    private long waitMillis = DEFAULT_WAIT_MILLIS;

    @Override
    public DBLockProvider create(KeycloakSession session) {
        return new SqliteLockProvider(locks, waitMillis);
    }

    @Override
    public void init(Config.Scope config) {
        waitMillis = config.getLong("lockWaitTimeoutMillis", DEFAULT_WAIT_MILLIS);
    }

    @Override
    public void postInit(KeycloakSessionFactory factory) {
    }

    @Override
    public void setTimeouts(long lockRecheckTimeMillis, long lockWaitTimeoutMillis) {
        waitMillis = lockWaitTimeoutMillis;
    }

    @Override
    public void close() {
    }

    @Override
    public String getId() {
        return PROVIDER_ID;
    }

    @Override
    public int order() {
        return PROVIDER_ORDER;
    }

    private static Map<DBLockProvider.Namespace, NamespaceLock> createLocks() {
        Map<DBLockProvider.Namespace, NamespaceLock> created = new EnumMap<>(DBLockProvider.Namespace.class);
        for (DBLockProvider.Namespace namespace : DBLockProvider.Namespace.values()) {
            created.put(namespace, new NamespaceLock());
        }
        return created;
    }
}
