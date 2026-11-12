package org.mesail.keycloak.passkey;

import com.webauthn4j.data.client.Origin;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.regex.Pattern;

public class PasskeyOrigins {

    private static final Pattern ALLOWED = Pattern.compile(
            "^(https://[a-z0-9]([a-z0-9.-]{0,251}[a-z0-9])?(:[0-9]{1,5})?"
                    + "|http://(localhost|127\\.0\\.0\\.1)(:[0-9]{1,5})?"
                    + "|android:apk-key-hash:[A-Za-z0-9_-]{16,128})$");
    private static final int MOST_ORIGINS = 16;
    private static final String SPACES = "\\s+";
    private static final String INVALID_REQUEST = "invalid_request";

    private PasskeyOrigins() {
    }

    public static List<String> split(String origins) {
        return List.of(textOrEmpty(origins).strip().split(SPACES));
    }

    public static Set<Origin> checked(List<String> origins) {
        requireCount(origins);
        Set<Origin> checked = new LinkedHashSet<>();
        origins.forEach(origin -> checked.add(checkedOrigin(origin)));
        return checked;
    }

    private static String textOrEmpty(String origins) {
        if (origins == null) {
            return "";
        }
        return origins;
    }

    private static void requireCount(List<String> origins) {
        if (origins == null || origins.isEmpty() || origins.size() > MOST_ORIGINS) {
            throw PasskeyRefused.badRequest(INVALID_REQUEST);
        }
    }

    private static Origin checkedOrigin(String origin) {
        if (origin == null || !ALLOWED.matcher(origin).matches()) {
            throw PasskeyRefused.badRequest(INVALID_REQUEST);
        }
        return new Origin(origin);
    }
}
