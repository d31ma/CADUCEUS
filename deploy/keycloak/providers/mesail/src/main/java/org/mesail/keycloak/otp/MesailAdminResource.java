package org.mesail.keycloak.otp;

import jakarta.ws.rs.Consumes;
import jakarta.ws.rs.PUT;
import jakarta.ws.rs.Path;
import jakarta.ws.rs.PathParam;
import jakarta.ws.rs.Produces;
import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

public class MesailAdminResource {

    private final TotpEnrollment enrollment;

    public MesailAdminResource(TotpEnrollment enrollment) {
        this.enrollment = enrollment;
    }

    @PUT
    @Path("users/{userId}/otp")
    @Consumes(MediaType.APPLICATION_JSON)
    @Produces(MediaType.APPLICATION_JSON)
    public Response storeOtp(@PathParam("userId") String userId, OtpRequest request) {
        return enrollment.enroll(userId, request);
    }
}
