package org.mesail.keycloak.passkey;

import java.util.regex.Pattern;
import org.keycloak.common.util.Base64Url;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.RealmModel;

public class PasskeyChallenge {

    private static final Pattern BASE64URL_16_TO_64_BYTES = Pattern.compile("^[A-Za-z0-9_-]{22,86}$");
    private static final long LIFETIME_SECONDS = 300L;
    private static final String KEY_PREFIX = "mesail-passkey-challenge:";
    private static final String INVALID_REQUEST = "invalid_request";
    private static final String CHALLENGE_USED = "challenge_used";

    private PasskeyChallenge() {
    }

    public static byte[] decoded(String challenge) {
        requireShape(challenge);
        return Base64Url.decode(challenge);
    }

    public static void claim(KeycloakSession session, RealmModel realm, String challenge) {
        String key = KEY_PREFIX + realm.getId() + ":" + challenge;
        requireFirstUse(session.singleUseObjects().putIfAbsent(key, LIFETIME_SECONDS));
    }

    private static void requireShape(String challenge) {
        if (challenge == null || !BASE64URL_16_TO_64_BYTES.matcher(challenge).matches()) {
            throw PasskeyRefused.badRequest(INVALID_REQUEST);
        }
    }

    private static void requireFirstUse(boolean firstUse) {
        if (!firstUse) {
            throw PasskeyRefused.badRequest(CHALLENGE_USED);
        }
    }
}
