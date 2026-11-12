package org.mesail.keycloak.passkey;

import org.keycloak.credential.CredentialModel;
import org.keycloak.events.admin.OperationType;
import org.keycloak.events.admin.ResourceType;
import org.keycloak.models.UserModel;
import org.keycloak.services.resources.admin.AdminEventBuilder;

public class PasskeyEvents {

    private final AdminEventBuilder adminEvent;

    public PasskeyEvents(AdminEventBuilder adminEvent) {
        this.adminEvent = adminEvent;
    }

    public void record(OperationType operation, UserModel user, CredentialModel credential) {
        adminEvent.operation(operation).resource(ResourceType.USER)
                .resourcePath("users", user.getId(), "credentials", credential.getId())
                .representation(new PasskeyAudit(PasskeyUsers.TYPE, credential.getId(), credential.getUserLabel()))
                .success();
    }
}
