package org.mesail.keycloak.sqlite.lock;

import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;

public class NamespaceLock {

    private final Semaphore permit = new Semaphore(1, true);

    public void acquire(long waitMillis) {
        boolean acquired = tryAcquire(waitMillis);
        failWhenNotAcquired(acquired, waitMillis);
    }

    public void release() {
        permit.release();
    }

    private boolean tryAcquire(long waitMillis) {
        try {
            return permit.tryAcquire(waitMillis, TimeUnit.MILLISECONDS);
        } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("Interrupted while waiting for the database lock", interrupted);
        }
    }

    private void failWhenNotAcquired(boolean acquired, long waitMillis) {
        if (!acquired) {
            throw new IllegalStateException("Database lock not acquired within " + waitMillis + " ms");
        }
    }
}
