package org.mesail.keycloak.passkey;

import com.webauthn4j.data.AuthenticationRequest;
import com.webauthn4j.data.client.Origin;
import com.webauthn4j.server.ServerProperty;
import com.webauthn4j.util.exception.WebAuthnException;
import jakarta.ws.rs.core.MultivaluedMap;
import jakarta.ws.rs.core.Response;
import java.nio.charset.StandardCharsets;
import java.util.Set;
import org.jboss.logging.Logger;
import org.keycloak.authentication.AuthenticationFlowContext;
import org.keycloak.authentication.AuthenticationFlowError;
import org.keycloak.authentication.authenticators.directgrant.AbstractDirectGrantAuthenticator;
import org.keycloak.authentication.authenticators.util.AuthenticatorUtils;
import org.keycloak.credential.WebAuthnCredentialModelInput;
import org.keycloak.events.Errors;
import org.keycloak.models.UserModel;

public class PasskeySignIn {

    public static final String ASSERTION = "passkey_assertion";
    public static final String CHALLENGE = "passkey_challenge";
    public static final String ORIGINS = "passkey_origins";
    private static final Logger LOG = Logger.getLogger(PasskeySignIn.class);
    private static final String INVALID_GRANT = "invalid_grant";
    private static final String INVALID_CREDENTIALS = "Invalid user credentials";
    private static final String UNKNOWN_USER = "unknown_user";
    private static final String NOT_ALLOWED = "not_allowed";
    private static final String NOT_VALID = "not_valid";
    private static final boolean USER_VERIFICATION_REQUIRED = true;

    private final AuthenticationFlowContext context;
    private final AbstractDirectGrantAuthenticator grant;
    private final MultivaluedMap<String, String> form;

    public PasskeySignIn(AuthenticationFlowContext context, AbstractDirectGrantAuthenticator grant) {
        this.context = context;
        this.grant = grant;
        this.form = context.getHttpRequest().getDecodedFormParameters();
    }

    public void run() {
        signInWhenGiven();
    }

    private void signInWhenGiven() {
        if (!form.containsKey(ASSERTION)) {
            context.attempted();
            return;
        }
        signInOrRefuse();
    }

    private void signInOrRefuse() {
        try {
            signIn();
        } catch (PasskeyRefused refused) {
            refuse(refused.getMessage());
        }
    }

    private void signIn() {
        PasskeyJson assertion = PasskeyJson.read(form.getFirst(ASSERTION));
        Set<Origin> origins = PasskeyOrigins.checked(PasskeyOrigins.split(form.getFirst(ORIGINS)));
        String challenge = form.getFirst(CHALLENGE);
        ServerProperty server = PasskeyServer.property(origins, PasskeyRelyingParty.id(context.getRealm()), PasskeyChallenge.decoded(challenge));
        UserModel user = knownUser(assertion.responseBytes("userHandle"));
        context.setUser(user);
        context.getEvent().user(user);
        requireAllowed(user);
        PasskeyChallenge.claim(context.getSession(), context.getRealm(), challenge);
        WebAuthnCredentialModelInput input = input(assertion, server);
        requireValid(user, input);
        recordUse(user, assertion.credentialId());
        new PasskeyLockout(context).succeeded(user);
        context.success();
    }

    private void recordUse(UserModel user, byte[] webAuthnId) {
        PasskeyUsers users = new PasskeyUsers(context.getSession(), context.getRealm());
        PasskeyLastUse.record(user, users.passkeyWithWebAuthnId(user, webAuthnId).getId());
    }

    private UserModel knownUser(byte[] userHandle) {
        UserModel user = context.getSession().users().getUserById(context.getRealm(), new String(userHandle, StandardCharsets.UTF_8));
        if (user == null) {
            throw PasskeyRefused.badRequest(UNKNOWN_USER);
        }
        return user;
    }

    private void requireAllowed(UserModel user) {
        if (!user.isEnabled() || AuthenticatorUtils.getDisabledByBruteForceEventError(context, user) != null) {
            throw PasskeyRefused.badRequest(NOT_ALLOWED);
        }
    }

    private WebAuthnCredentialModelInput input(PasskeyJson assertion, ServerProperty server) {
        AuthenticationRequest request = new AuthenticationRequest(assertion.credentialId(), assertion.responseBytes("authenticatorData"),
                assertion.responseBytes("clientDataJSON"), assertion.responseBytes("signature"));
        WebAuthnCredentialModelInput input = new WebAuthnCredentialModelInput(PasskeyUsers.TYPE);
        input.setAuthenticationRequest(request);
        input.setAuthenticationParameters(new WebAuthnCredentialModelInput.KeycloakWebAuthnAuthenticationParameters(server, USER_VERIFICATION_REQUIRED));
        return input;
    }

    private void requireValid(UserModel user, WebAuthnCredentialModelInput input) {
        if (!checked(user, input)) {
            throw PasskeyRefused.badRequest(NOT_VALID);
        }
    }

    private boolean checked(UserModel user, WebAuthnCredentialModelInput input) {
        try {
            return user.credentialManager().isValid(input);
        } catch (WebAuthnException error) {
            LOG.debugf("Passkey check failed: %s", error.getClass().getSimpleName());
            return false;
        }
    }

    private void refuse(String reason) {
        LOG.debugf("Passkey sign-in refused in realm %s: %s", context.getRealm().getName(), reason);
        new PasskeyLockout(context).failed();
        context.getEvent().error(Errors.INVALID_USER_CREDENTIALS);
        Response response = grant.errorResponse(Response.Status.BAD_REQUEST.getStatusCode(), INVALID_GRANT, INVALID_CREDENTIALS);
        context.failure(AuthenticationFlowError.INVALID_USER, response);
    }
}
