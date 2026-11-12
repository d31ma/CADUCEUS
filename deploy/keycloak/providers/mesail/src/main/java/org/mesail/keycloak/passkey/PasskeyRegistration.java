package org.mesail.keycloak.passkey;

import com.webauthn4j.WebAuthnRegistrationManager;
import com.webauthn4j.data.RegistrationData;
import com.webauthn4j.data.RegistrationParameters;
import com.webauthn4j.data.RegistrationRequest;
import com.webauthn4j.server.ServerProperty;
import com.webauthn4j.util.exception.WebAuthnException;
import jakarta.ws.rs.core.Response;
import org.jboss.logging.Logger;
import org.keycloak.common.util.Base64Url;
import org.keycloak.credential.CredentialModel;
import org.keycloak.credential.CredentialProvider;
import org.keycloak.credential.WebAuthnCredentialModelInput;
import org.keycloak.credential.WebAuthnCredentialProvider;
import org.keycloak.credential.WebAuthnPasswordlessCredentialProviderFactory;
import org.keycloak.events.admin.OperationType;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.ModelDuplicateException;
import org.keycloak.models.RealmModel;
import org.keycloak.models.UserModel;
import org.keycloak.models.credential.WebAuthnCredentialModel;
import org.keycloak.services.resources.admin.AdminEventBuilder;
import org.keycloak.services.resources.admin.fgap.AdminPermissionEvaluator;

public class PasskeyRegistration {

    private static final Logger LOG = Logger.getLogger(PasskeyRegistration.class);
    private static final int MOST_PASSKEYS = 10;
    private static final boolean USER_VERIFICATION_REQUIRED = true;
    private static final boolean USER_PRESENCE_REQUIRED = true;
    private static final String INVALID_REQUEST = "invalid_request";
    private static final String INVALID_PASSKEY = "invalid_passkey";
    private static final String TOO_MANY = "too_many";
    private static final String PASSKEY_EXISTS = "passkey_exists";
    private static final String LABEL_EXISTS = "label_exists";

    private final KeycloakSession session;
    private final RealmModel realm;
    private final AdminPermissionEvaluator auth;
    private final PasskeyEvents events;
    private final PasskeyUsers users;

    public PasskeyRegistration(KeycloakSession session, RealmModel realm, AdminPermissionEvaluator auth, AdminEventBuilder adminEvent) {
        this.session = session;
        this.realm = realm;
        this.auth = auth;
        this.events = new PasskeyEvents(adminEvent);
        this.users = new PasskeyUsers(session, realm);
    }

    public Response register(String userId, PasskeyRegistrationRequest request) {
        try {
            return registered(userId, request);
        } catch (PasskeyRefused refused) {
            return refused.response();
        }
    }

    private Response registered(String userId, PasskeyRegistrationRequest request) {
        UserModel user = users.existing(userId);
        auth.users().requireManage(user);
        requireBody(request);
        String label = PasskeyLabel.checked(request.label);
        ServerProperty server = PasskeyServer.property(PasskeyOrigins.checked(request.origins), PasskeyRelyingParty.id(realm),
                PasskeyChallenge.decoded(request.challenge));
        PasskeyJson attestation = PasskeyJson.read(request.credentialJson);
        requireRoom(user);
        PasskeyChallenge.claim(session, realm, request.challenge);
        RegistrationData data = verified(attestation, server);
        requireNew(user, data);
        CredentialModel stored = store(user, data, label);
        events.record(OperationType.CREATE, user, stored);
        LOG.infof("Stored a passkey for user %s in realm %s", user.getId(), realm.getName());
        return Response.status(Response.Status.CREATED).entity(users.view(user, stored)).build();
    }

    private static void requireBody(PasskeyRegistrationRequest request) {
        if (request == null) {
            throw PasskeyRefused.badRequest(INVALID_REQUEST);
        }
    }

    private void requireRoom(UserModel user) {
        if (users.passkeys(user).size() >= MOST_PASSKEYS) {
            throw new PasskeyRefused(Response.Status.CONFLICT, TOO_MANY);
        }
    }

    private RegistrationData verified(PasskeyJson attestation, ServerProperty server) {
        RegistrationRequest request = new RegistrationRequest(attestation.responseBytes("attestationObject"),
                attestation.responseBytes("clientDataJSON"), attestation.transports());
        RegistrationParameters parameters = new RegistrationParameters(server,
                PasskeyAlgorithms.fromPolicy(realm.getWebAuthnPolicyPasswordless()), USER_VERIFICATION_REQUIRED, USER_PRESENCE_REQUIRED);
        try {
            WebAuthnRegistrationManager manager = WebAuthnRegistrationManager.createNonStrictWebAuthnRegistrationManager();
            return manager.verify(manager.parse(request), parameters);
        } catch (WebAuthnException error) {
            LOG.debugf("Passkey registration refused: %s", error.getClass().getSimpleName());
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private void requireNew(UserModel user, RegistrationData data) {
        String credentialId = Base64Url.encode(data.getAttestationObject().getAuthenticatorData().getAttestedCredentialData().getCredentialId());
        if (users.passkeys(user).stream().anyMatch(credential -> PasskeyUsers.webAuthnId(credential).equals(credentialId))) {
            throw new PasskeyRefused(Response.Status.CONFLICT, PASSKEY_EXISTS);
        }
    }

    private CredentialModel store(UserModel user, RegistrationData data, String label) {
        var authenticatorData = data.getAttestationObject().getAuthenticatorData();
        WebAuthnCredentialModelInput input = new WebAuthnCredentialModelInput(PasskeyUsers.TYPE);
        input.setAttestedCredentialData(authenticatorData.getAttestedCredentialData());
        input.setCount(authenticatorData.getSignCount());
        input.setAttestationStatementFormat(data.getAttestationObject().getFormat());
        input.setTransports(data.getTransports());
        WebAuthnCredentialProvider provider = (WebAuthnCredentialProvider) session.getProvider(CredentialProvider.class,
                WebAuthnPasswordlessCredentialProviderFactory.PROVIDER_ID);
        return created(user, provider, provider.getCredentialModelFromCredentialInput(input, label));
    }

    private CredentialModel created(UserModel user, WebAuthnCredentialProvider provider, WebAuthnCredentialModel model) {
        try {
            return provider.createCredential(realm, user, model);
        } catch (ModelDuplicateException error) {
            throw new PasskeyRefused(Response.Status.CONFLICT, LABEL_EXISTS);
        }
    }
}
