package org.mesail.keycloak.passkey;

import jakarta.ws.rs.core.MediaType;
import jakarta.ws.rs.core.Response;

public class PasskeyRefused extends RuntimeException {

    private static final long serialVersionUID = 1L;

    private final Response.Status status;
    private final String error;

    public PasskeyRefused(Response.Status status, String error) {
        super(error);
        this.status = status;
        this.error = error;
    }

    public static PasskeyRefused badRequest(String error) {
        return new PasskeyRefused(Response.Status.BAD_REQUEST, error);
    }

    public Response response() {
        return Response.status(status).type(MediaType.APPLICATION_JSON).entity(new PasskeyProblem(error)).build();
    }
}
