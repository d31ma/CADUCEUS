package org.mesail.keycloak.otp;

import jakarta.ws.rs.core.Response;
import org.jboss.logging.Logger;
import org.keycloak.credential.CredentialModel;
import org.keycloak.events.admin.OperationType;
import org.keycloak.events.admin.ResourceType;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;
import org.keycloak.models.UserCredentialModel;
import org.keycloak.models.UserModel;
import org.keycloak.models.credential.OTPCredentialModel;
import org.keycloak.services.resources.admin.AdminEventBuilder;
import org.keycloak.services.resources.admin.fgap.AdminPermissionEvaluator;

public class TotpEnrollment {

    private static final Logger LOG = Logger.getLogger(TotpEnrollment.class);
    private static final String SECRET_ENCODING = "BASE32";
    private static final String USER_LABEL = "Mesail";
    private static final String CODE_KEY_PREFIX = "mesail-otp-confirm:";
    private static final String NOT_FOUND = "user_not_found";
    private static final String OTP_EXISTS = "otp_exists";
    private static final String CODE_USED = "code_used";
    private static final String INVALID_CODE = "invalid_code";

    private final KeycloakSession session;
    private final RealmModel realm;
    private final AdminPermissionEvaluator auth;
    private final AdminEventBuilder adminEvent;
    private final OtpRequestCheck check;

    public TotpEnrollment(KeycloakSession session, RealmModel realm, AdminPermissionEvaluator auth, AdminEventBuilder adminEvent) {
        this.session = session;
        this.realm = realm;
        this.auth = auth;
        this.adminEvent = adminEvent;
        this.check = new OtpRequestCheck(realm.getOTPPolicy());
    }

    public Response enroll(String userId, OtpRequest request) {
        try {
            return enrolled(userId, request);
        } catch (OtpRefused refused) {
            return refused.response();
        }
    }

    private Response enrolled(String userId, OtpRequest request) {
        UserModel user = existingUser(userId);
        auth.users().requireManage(user);
        refuseExistingOtp(user);
        byte[] secret = check.checkedSecret(request);
        check.checkCode(request, secret);
        claimCode(user, request.code);
        CredentialModel credential = storeCredential(user, request);
        markCodeUsedForSignIn(user, credential, request.code);
        audit(user, credential);
        return Response.noContent().build();
    }

    private UserModel existingUser(String userId) {
        UserModel user = session.users().getUserById(realm, userId);
        if (user == null) {
            throw new OtpRefused(Response.Status.NOT_FOUND, NOT_FOUND);
        }
        return user;
    }

    private void refuseExistingOtp(UserModel user) {
        if (user.credentialManager().getStoredCredentialsByTypeStream(OTPCredentialModel.TYPE).findAny().isPresent()) {
            throw new OtpRefused(Response.Status.CONFLICT, OTP_EXISTS);
        }
    }

    private void claimCode(UserModel user, String code) {
        String key = CODE_KEY_PREFIX + realm.getId() + ":" + user.getId() + ":" + code;
        if (!session.singleUseObjects().putIfAbsent(key, check.codeLifetimeSeconds())) {
            throw new OtpRefused(Response.Status.BAD_REQUEST, CODE_USED);
        }
    }

    private CredentialModel storeCredential(UserModel user, OtpRequest request) {
        OTPCredentialModel model = OTPCredentialModel.createTOTP(request.secret, request.digits, request.period, request.algorithm, SECRET_ENCODING);
        model.setUserLabel(USER_LABEL);
        return user.credentialManager().createStoredCredential(model);
    }

    private void markCodeUsedForSignIn(UserModel user, CredentialModel credential, String code) {
        boolean accepted = user.credentialManager().isValid(new UserCredentialModel(credential.getId(), OTPCredentialModel.TYPE, code));
        if (!accepted) {
            user.credentialManager().removeStoredCredentialById(credential.getId());
            throw new OtpRefused(Response.Status.BAD_REQUEST, INVALID_CODE);
        }
    }

    private void audit(UserModel user, CredentialModel credential) {
        adminEvent.operation(OperationType.CREATE).resource(ResourceType.USER)
                .resourcePath("users", user.getId(), "credentials", credential.getId())
                .representation(new OtpAudit(OTPCredentialModel.TYPE, credential.getId(), USER_LABEL))
                .success();
        LOG.infof("Stored a TOTP credential for user %s in realm %s", user.getId(), realm.getName());
    }
}
