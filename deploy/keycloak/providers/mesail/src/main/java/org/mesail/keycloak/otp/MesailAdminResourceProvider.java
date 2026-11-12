package org.mesail.keycloak.otp;

import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;
import org.keycloak.services.resources.admin.AdminEventBuilder;
import org.keycloak.services.resources.admin.ext.AdminRealmResourceProvider;
import org.keycloak.services.resources.admin.fgap.AdminPermissionEvaluator;
import org.mesail.keycloak.passkey.PasskeyAccount;
import org.mesail.keycloak.passkey.PasskeyRegistration;

public class MesailAdminResourceProvider implements AdminRealmResourceProvider {

    @Override
    public Object getResource(KeycloakSession session, RealmModel realm, AdminPermissionEvaluator auth, AdminEventBuilder adminEvent) {
        return new MesailAdminResource(new TotpEnrollment(session, realm, auth, adminEvent),
                new PasskeyRegistration(session, realm, auth, adminEvent), new PasskeyAccount(session, realm, auth, adminEvent));
    }

    @Override
    public void close() {
    }
}
