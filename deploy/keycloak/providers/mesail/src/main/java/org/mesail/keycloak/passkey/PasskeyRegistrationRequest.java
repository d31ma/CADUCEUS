package org.mesail.keycloak.passkey;

import java.util.List;

public class PasskeyRegistrationRequest {

    public String credentialJson;
    public String challenge;
    public List<String> origins;
    public String label;
}
