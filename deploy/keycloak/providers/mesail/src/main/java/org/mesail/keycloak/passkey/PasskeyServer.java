package org.mesail.keycloak.passkey;

import com.webauthn4j.data.client.Origin;
import com.webauthn4j.data.client.challenge.Challenge;
import com.webauthn4j.data.client.challenge.DefaultChallenge;
import com.webauthn4j.server.ServerProperty;
import java.util.Set;

public class PasskeyServer {

    private PasskeyServer() {
    }

    @SuppressWarnings("deprecation")
    public static ServerProperty property(Set<Origin> origins, String relyingPartyId, byte[] challenge) {
        Challenge expected = new DefaultChallenge(challenge);
        return new ServerProperty(origins, relyingPartyId, expected);
    }
}
