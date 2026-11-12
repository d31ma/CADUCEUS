package org.mesail.keycloak.passkey;

import java.util.ArrayList;
import java.util.List;
import org.keycloak.common.util.Time;
import org.keycloak.models.UserModel;

public class PasskeyLastUse {

    public static final String ATTRIBUTE = "mesail.passkeyLastUsed";
    private static final String SEPARATOR = " ";

    private PasskeyLastUse() {
    }

    public static void record(UserModel user, String passkeyId) {
        List<String> values = valuesOfOthers(user, passkeyId);
        values.add(passkeyId + SEPARATOR + Time.currentTimeMillis());
        store(user, values);
    }

    public static Long lastUsed(UserModel user, String passkeyId) {
        String prefix = passkeyId + SEPARATOR;
        return user.getAttributeStream(ATTRIBUTE)
                .filter(value -> value.startsWith(prefix))
                .map(value -> millis(value.substring(prefix.length())))
                .findFirst()
                .orElse(null);
    }

    public static void forget(UserModel user, String passkeyId) {
        store(user, valuesOfOthers(user, passkeyId));
    }

    private static List<String> valuesOfOthers(UserModel user, String passkeyId) {
        String prefix = passkeyId + SEPARATOR;
        return new ArrayList<>(user.getAttributeStream(ATTRIBUTE).filter(value -> !value.startsWith(prefix)).toList());
    }

    private static void store(UserModel user, List<String> values) {
        if (values.isEmpty()) {
            user.removeAttribute(ATTRIBUTE);
            return;
        }
        user.setAttribute(ATTRIBUTE, values);
    }

    private static Long millis(String text) {
        try {
            return Long.valueOf(text);
        } catch (NumberFormatException error) {
            return null;
        }
    }
}
