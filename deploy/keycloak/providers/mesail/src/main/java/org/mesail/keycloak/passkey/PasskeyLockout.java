package org.mesail.keycloak.passkey;

import java.util.Set;
import org.keycloak.authentication.AuthenticationFlowContext;
import org.keycloak.models.UserModel;
import org.keycloak.models.credential.PasswordCredentialModel;
import org.keycloak.services.managers.BruteForceProtector;

public class PasskeyLockout {

    private static final Set<String> SAME_LOCKOUT_AS_PASSWORD = Set.of(PasswordCredentialModel.TYPE);

    private final AuthenticationFlowContext context;

    public PasskeyLockout(AuthenticationFlowContext context) {
        this.context = context;
    }

    public void failed() {
        UserModel user = context.getUser();
        if (user == null) {
            return;
        }
        protector().failedLogin(context.getRealm(), user, context.getConnection(), context.getUriInfo(), SAME_LOCKOUT_AS_PASSWORD);
    }

    public void succeeded(UserModel user) {
        protector().successfulLogin(context.getRealm(), user, context.getConnection(), context.getUriInfo(), SAME_LOCKOUT_AS_PASSWORD);
    }

    private BruteForceProtector protector() {
        return context.getSession().getProvider(BruteForceProtector.class);
    }
}
