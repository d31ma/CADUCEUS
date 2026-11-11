package org.mesail.keycloak.otp;

public record OtpAudit(String type, String credentialId, String userLabel) {
}
