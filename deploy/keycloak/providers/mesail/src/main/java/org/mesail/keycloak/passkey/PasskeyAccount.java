package org.mesail.keycloak.passkey;

import jakarta.ws.rs.core.Response;
import java.util.List;
import org.jboss.logging.Logger;
import org.keycloak.credential.CredentialModel;
import org.keycloak.events.admin.OperationType;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.ModelDuplicateException;
import org.keycloak.models.RealmModel;
import org.keycloak.models.UserModel;
import org.keycloak.services.resources.admin.AdminEventBuilder;
import org.keycloak.services.resources.admin.fgap.AdminPermissionEvaluator;

public class PasskeyAccount {

    private static final Logger LOG = Logger.getLogger(PasskeyAccount.class);
    private static final String LABEL_EXISTS = "label_exists";

    private final RealmModel realm;
    private final AdminPermissionEvaluator auth;
    private final PasskeyEvents events;
    private final PasskeyUsers users;

    public PasskeyAccount(KeycloakSession session, RealmModel realm, AdminPermissionEvaluator auth, AdminEventBuilder adminEvent) {
        this.realm = realm;
        this.auth = auth;
        this.events = new PasskeyEvents(adminEvent);
        this.users = new PasskeyUsers(session, realm);
    }

    public Response list(String userId) {
        try {
            return Response.ok(listed(userId)).build();
        } catch (PasskeyRefused refused) {
            return refused.response();
        }
    }

    public Response rename(String userId, String passkeyId, PasskeyRenameRequest request) {
        try {
            return Response.ok(renamed(userId, passkeyId, request)).build();
        } catch (PasskeyRefused refused) {
            return refused.response();
        }
    }

    public Response remove(String userId, String passkeyId) {
        try {
            removed(userId, passkeyId);
            return Response.noContent().build();
        } catch (PasskeyRefused refused) {
            return refused.response();
        }
    }

    public Response removeAll(String userId) {
        try {
            allRemoved(userId);
            return Response.noContent().build();
        } catch (PasskeyRefused refused) {
            return refused.response();
        }
    }

    private List<PasskeyView> listed(String userId) {
        UserModel user = users.existing(userId);
        auth.users().requireView(user);
        return users.passkeys(user).stream().map(credential -> users.view(user, credential)).toList();
    }

    private PasskeyView renamed(String userId, String passkeyId, PasskeyRenameRequest request) {
        UserModel user = users.existing(userId);
        auth.users().requireManage(user);
        CredentialModel passkey = users.passkey(user, passkeyId);
        String label = PasskeyLabel.checked(request == null ? null : request.label);
        relabel(user, passkey, label);
        CredentialModel renamed = users.passkey(user, passkeyId);
        events.record(OperationType.UPDATE, user, renamed);
        return users.view(user, renamed);
    }

    private void relabel(UserModel user, CredentialModel passkey, String label) {
        try {
            user.credentialManager().updateCredentialLabel(passkey.getId(), label);
        } catch (ModelDuplicateException error) {
            throw new PasskeyRefused(Response.Status.CONFLICT, LABEL_EXISTS);
        }
    }

    private void removed(String userId, String passkeyId) {
        UserModel user = users.existing(userId);
        auth.users().requireManage(user);
        delete(user, users.passkey(user, passkeyId));
    }

    private void allRemoved(String userId) {
        UserModel user = users.existing(userId);
        auth.users().requireManage(user);
        users.passkeys(user).forEach(passkey -> delete(user, passkey));
    }

    private void delete(UserModel user, CredentialModel passkey) {
        user.credentialManager().removeStoredCredentialById(passkey.getId());
        PasskeyLastUse.forget(user, passkey.getId());
        events.record(OperationType.DELETE, user, passkey);
        LOG.infof("Removed a passkey of user %s in realm %s", user.getId(), realm.getName());
    }
}
