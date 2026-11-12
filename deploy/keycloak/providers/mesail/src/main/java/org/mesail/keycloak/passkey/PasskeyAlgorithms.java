package org.mesail.keycloak.passkey;

import com.webauthn4j.data.PublicKeyCredentialParameters;
import com.webauthn4j.data.PublicKeyCredentialType;
import com.webauthn4j.data.attestation.statement.COSEAlgorithmIdentifier;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.keycloak.models.WebAuthnPolicy;

public class PasskeyAlgorithms {

    private static final Map<String, COSEAlgorithmIdentifier> KNOWN = Map.of(
            "ES256", COSEAlgorithmIdentifier.ES256,
            "RS256", COSEAlgorithmIdentifier.RS256);

    private PasskeyAlgorithms() {
    }

    public static List<PublicKeyCredentialParameters> fromPolicy(WebAuthnPolicy policy) {
        List<PublicKeyCredentialParameters> parameters = new ArrayList<>();
        policy.getSignatureAlgorithm().forEach(name -> addKnown(parameters, name));
        return parameters;
    }

    private static void addKnown(List<PublicKeyCredentialParameters> parameters, String name) {
        if (KNOWN.containsKey(name)) {
            parameters.add(new PublicKeyCredentialParameters(PublicKeyCredentialType.PUBLIC_KEY, KNOWN.get(name)));
        }
    }
}
