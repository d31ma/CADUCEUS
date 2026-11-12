package org.mesail.keycloak.otp;

import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.DELETE;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.PUT;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;
import org.mesail.keycloak.passkey.PasskeyAccount;
import org.mesail.keycloak.passkey.PasskeyRegistration;
import org.mesail.keycloak.passkey.PasskeyRegistrationRequest;
import org.mesail.keycloak.passkey.PasskeyRenameRequest;

public class MesailAdminResource {

    private final TotpEnrollment enrollment;
    private final PasskeyRegistration registration;
    private final PasskeyAccount passkeys;

    public MesailAdminResource(TotpEnrollment enrollment, PasskeyRegistration registration, PasskeyAccount passkeys) {
        this.enrollment = enrollment;
        this.registration = registration;
        this.passkeys = passkeys;
    }

    @PUT
    @Path("users/{userId}/otp")
    @Consumes(MediaType.APPLICATION_JSON)
    @Produces(MediaType.APPLICATION_JSON)
    public Response storeOtp(@PathParam("userId") String userId, OtpRequest request) {
        return enrollment.enroll(userId, request);
    }

    @GET
    @Path("users/{userId}/passkeys")
    @Produces(MediaType.APPLICATION_JSON)
    public Response listPasskeys(@PathParam("userId") String userId) {
        return passkeys.list(userId);
    }

    @POST
    @Path("users/{userId}/passkeys")
    @Consumes(MediaType.APPLICATION_JSON)
    @Produces(MediaType.APPLICATION_JSON)
    public Response registerPasskey(@PathParam("userId") String userId, PasskeyRegistrationRequest request) {
        return registration.register(userId, request);
    }

    @PUT
    @Path("users/{userId}/passkeys/{passkeyId}")
    @Consumes(MediaType.APPLICATION_JSON)
    @Produces(MediaType.APPLICATION_JSON)
    public Response renamePasskey(@PathParam("userId") String userId, @PathParam("passkeyId") String passkeyId, PasskeyRenameRequest request) {
        return passkeys.rename(userId, passkeyId, request);
    }

    @DELETE
    @Path("users/{userId}/passkeys/{passkeyId}")
    @Produces(MediaType.APPLICATION_JSON)
    public Response removePasskey(@PathParam("userId") String userId, @PathParam("passkeyId") String passkeyId) {
        return passkeys.remove(userId, passkeyId);
    }

    @DELETE
    @Path("users/{userId}/passkeys")
    @Produces(MediaType.APPLICATION_JSON)
    public Response removeAllPasskeys(@PathParam("userId") String userId) {
        return passkeys.removeAll(userId);
    }
}
