package org.mesail.keycloak.passkey;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.util.LinkedHashSet;
import java.util.Set;
import java.util.regex.Pattern;
import org.keycloak.common.util.Base64Url;
import org.keycloak.util.JsonSerialization;

public class PasskeyJson {

    private static final int LARGEST_JSON = 65536;
    private static final int MOST_TRANSPORTS = 8;
    private static final Pattern BASE64URL = Pattern.compile("^[A-Za-z0-9_-]+$");
    private static final Pattern TRANSPORT = Pattern.compile("^[a-z-]{1,32}$");
    private static final String INVALID_PASSKEY = "invalid_passkey";
    private static final String PUBLIC_KEY = "public-key";

    private final JsonNode credential;
    private final JsonNode response;

    private PasskeyJson(JsonNode credential) {
        this.credential = credential;
        this.response = credential.path("response");
    }

    public static PasskeyJson read(String json) {
        requireSize(json);
        PasskeyJson passkey = new PasskeyJson(parsedTree(json));
        passkey.requirePublicKeyType();
        passkey.requireSameIds();
        return passkey;
    }

    public byte[] credentialId() {
        return decoded(credential, "rawId");
    }

    public byte[] responseBytes(String field) {
        return decoded(response, field);
    }

    public Set<String> transports() {
        JsonNode list = response.path("transports");
        requireTransportCount(list);
        Set<String> transports = new LinkedHashSet<>();
        list.forEach(node -> transports.add(checkedTransport(node)));
        return transports;
    }

    private static void requireSize(String json) {
        if (json == null || json.isEmpty() || json.length() > LARGEST_JSON) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private static JsonNode parsedTree(String json) {
        try {
            return JsonSerialization.mapper.readTree(json);
        } catch (IOException error) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private void requirePublicKeyType() {
        if (!PUBLIC_KEY.equals(credential.path("type").asText())) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private void requireSameIds() {
        if (!credential.path("id").asText().equals(credential.path("rawId").asText())) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private static void requireTransportCount(JsonNode list) {
        if (!list.isMissingNode() && (!list.isArray() || list.size() > MOST_TRANSPORTS)) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
    }

    private static String checkedTransport(JsonNode node) {
        if (!node.isTextual() || !TRANSPORT.matcher(node.asText()).matches()) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
        return node.asText();
    }

    private static byte[] decoded(JsonNode parent, String field) {
        String text = parent.path(field).asText("");
        if (!BASE64URL.matcher(text).matches()) {
            throw PasskeyRefused.badRequest(INVALID_PASSKEY);
        }
        return Base64Url.decode(text);
    }
}
