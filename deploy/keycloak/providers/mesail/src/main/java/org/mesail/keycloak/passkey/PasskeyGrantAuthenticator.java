package org.mesail.keycloak.passkey;

import java.util.List;
import org.keycloak.authentication.AuthenticationFlowContext;
import org.keycloak.authentication.authenticators.directgrant.AbstractDirectGrantAuthenticator;
import org.keycloak.models.AuthenticationExecutionModel;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;
import org.keycloak.models.UserModel;
import org.keycloak.provider.ProviderConfigProperty;

public class PasskeyGrantAuthenticator extends AbstractDirectGrantAuthenticator {

    public static final String PROVIDER_ID = "mesail-passkey-grant";
    private static final AuthenticationExecutionModel.Requirement[] REQUIREMENTS = {
        AuthenticationExecutionModel.Requirement.ALTERNATIVE,
        AuthenticationExecutionModel.Requirement.DISABLED,
    };

    @Override
    public void authenticate(AuthenticationFlowContext context) {
        new PasskeySignIn(context, this).run();
    }

    @Override
    public boolean requiresUser() {
        return false;
    }

    @Override
    public boolean configuredFor(KeycloakSession session, RealmModel realm, UserModel user) {
        return true;
    }

    @Override
    public void setRequiredActions(KeycloakSession session, RealmModel realm, UserModel user) {
    }

    @Override
    public String getDisplayType() {
        return "Mesail passkey";
    }

    @Override
    public String getReferenceCategory() {
        return PasskeyUsers.TYPE;
    }

    @Override
    public boolean isConfigurable() {
        return false;
    }

    @Override
    public AuthenticationExecutionModel.Requirement[] getRequirementChoices() {
        return REQUIREMENTS.clone();
    }

    @Override
    public boolean isUserSetupAllowed() {
        return false;
    }

    @Override
    public String getHelpText() {
        return "Signs a person in with a passkey assertion that the Mesail identity service passes in the "
                + PasskeySignIn.ASSERTION + ", " + PasskeySignIn.CHALLENGE + " and " + PasskeySignIn.ORIGINS
                + " form parameters (the allowed origins separated by spaces). Without them the next alternative runs.";
    }

    @Override
    public List<ProviderConfigProperty> getConfigProperties() {
        return List.of();
    }

    @Override
    public String getId() {
        return PROVIDER_ID;
    }
}
