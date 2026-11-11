package org.mesail.keycloak.sqlite.lock;

import java.util.Map;
import org.keycloak.models.dblock.DBLockProvider;

public class SqliteLockProvider implements DBLockProvider {

    private final Map<Namespace, NamespaceLock> locks;
    private final long waitMillis;
    private Namespace currentLock;

    public SqliteLockProvider(Map<Namespace, NamespaceLock> locks, long waitMillis) {
        this.locks = locks;
        this.waitMillis = waitMillis;
    }

    @Override
    public void waitForLock(Namespace namespace) {
        locks.get(namespace).acquire(waitMillis);
        currentLock = namespace;
    }

    @Override
    public void releaseLock() {
        releaseCurrentLock();
    }

    @Override
    public Namespace getCurrentLock() {
        return currentLock;
    }

    @Override
    public boolean supportsForcedUnlock() {
        return false;
    }

    @Override
    public void destroyLockInfo() {
        currentLock = null;
    }

    @Override
    public void close() {
        releaseCurrentLock();
    }

    private void releaseCurrentLock() {
        if (currentLock == null) {
            return;
        }
        locks.get(currentLock).release();
        currentLock = null;
    }
}
