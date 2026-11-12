package org.mesail.keycloak.passkey;

import java.util.regex.Pattern;

public class PasskeyLabel {

    private static final Pattern ALLOWED = Pattern.compile("^[^\\p{Cntrl}]{1,64}$");
    private static final String INVALID_REQUEST = "invalid_request";

    private PasskeyLabel() {
    }

    public static String checked(String label) {
        String trimmed = trimmedOrEmpty(label);
        requireAllowed(trimmed);
        return trimmed;
    }

    private static String trimmedOrEmpty(String label) {
        if (label == null) {
            return "";
        }
        return label.strip();
    }

    private static void requireAllowed(String label) {
        if (!ALLOWED.matcher(label).matches()) {
            throw PasskeyRefused.badRequest(INVALID_REQUEST);
        }
    }
}
