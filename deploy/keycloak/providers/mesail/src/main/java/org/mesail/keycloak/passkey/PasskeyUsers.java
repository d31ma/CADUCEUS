package org.mesail.keycloak.passkey;

import jakarta.ws.rs.core.Response;
import java.util.Base64;
import java.util.List;
import java.util.function.Predicate;
import org.keycloak.common.util.Base64Url;
import org.keycloak.credential.CredentialModel;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;
import org.keycloak.models.UserModel;
import org.keycloak.models.credential.WebAuthnCredentialModel;

public class PasskeyUsers {

    public static final String TYPE = WebAuthnCredentialModel.TYPE_PASSWORDLESS;
    private static final String USER_NOT_FOUND = "user_not_found";
    private static final String NOT_FOUND = "not_found";

    private final KeycloakSession session;
    private final RealmModel realm;

    public PasskeyUsers(KeycloakSession session, RealmModel realm) {
        this.session = session;
        this.realm = realm;
    }

    public UserModel existing(String userId) {
        UserModel user = session.users().getUserById(realm, userId);
        if (user == null) {
            throw new PasskeyRefused(Response.Status.NOT_FOUND, USER_NOT_FOUND);
        }
        return user;
    }

    public List<CredentialModel> passkeys(UserModel user) {
        return user.credentialManager().getStoredCredentialsByTypeStream(TYPE).toList();
    }

    public CredentialModel passkey(UserModel user, String passkeyId) {
        return onlyMatch(user, credential -> credential.getId().equals(passkeyId));
    }

    public CredentialModel passkeyWithWebAuthnId(UserModel user, byte[] webAuthnId) {
        String wanted = Base64Url.encode(webAuthnId);
        return onlyMatch(user, credential -> webAuthnId(credential).equals(wanted));
    }

    public PasskeyView view(UserModel user, CredentialModel credential) {
        return new PasskeyView(credential.getId(), webAuthnId(credential), credential.getUserLabel(),
                credential.getCreatedDate(), PasskeyLastUse.lastUsed(user, credential.getId()));
    }

    private CredentialModel onlyMatch(UserModel user, Predicate<CredentialModel> matches) {
        return passkeys(user).stream()
                .filter(matches)
                .findFirst()
                .orElseThrow(() -> new PasskeyRefused(Response.Status.NOT_FOUND, NOT_FOUND));
    }

    public static String webAuthnId(CredentialModel credential) {
        String stored = WebAuthnCredentialModel.createFromCredentialModel(credential).getWebAuthnCredentialData().getCredentialId();
        return Base64Url.encode(Base64.getDecoder().decode(stored));
    }
}
