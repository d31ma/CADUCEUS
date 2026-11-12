package org.mesail.keycloak.passkey;

public record PasskeyView(String passkeyId, String credentialId, String label, Long createdAt, Long lastUsedAt) {
}
