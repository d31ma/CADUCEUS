package org.mesail.keycloak.passkey;

import jakarta.ws.rs.core.Response;
import org.keycloak.models.RealmModel;

public class PasskeyRelyingParty {

    private static final String NOT_CONFIGURED = "passkeys_not_configured";

    private PasskeyRelyingParty() {
    }

    public static String id(RealmModel realm) {
        String id = realm.getWebAuthnPolicyPasswordless().getRpId();
        if (id == null || id.isBlank()) {
            throw new PasskeyRefused(Response.Status.SERVICE_UNAVAILABLE, NOT_CONFIGURED);
        }
        return id;
    }
}
