package org.mesail.keycloak.otp;

import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

public class OtpRefused extends RuntimeException {

    private static final long serialVersionUID = 1L;

    private final Response.Status status;
    private final String error;

    public OtpRefused(Response.Status status, String error) {
        super(error);
        this.status = status;
        this.error = error;
    }

    public Response response() {
        return Response.status(status).type(MediaType.APPLICATION_JSON).entity(new OtpProblem(error)).build();
    }
}
