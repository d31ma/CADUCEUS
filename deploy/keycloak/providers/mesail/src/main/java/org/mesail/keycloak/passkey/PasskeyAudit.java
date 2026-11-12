package org.mesail.keycloak.passkey;

public record PasskeyAudit(String type, String credentialId, String userLabel) {
}
