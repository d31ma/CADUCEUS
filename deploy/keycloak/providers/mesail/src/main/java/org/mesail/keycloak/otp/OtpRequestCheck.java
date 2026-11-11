package org.mesail.keycloak.otp;

import jakarta.ws.rs.core.Response;
import java.util.regex.Pattern;
import org.keycloak.models.OTPPolicy;
import org.keycloak.models.utils.Base32;
import org.keycloak.models.utils.TimeBasedOTP;

public class OtpRequestCheck {

    private static final Pattern BASE32_SECRET = Pattern.compile("^[A-Z2-7]{32,128}$");
    private static final Pattern DIGITS_ONLY = Pattern.compile("^[0-9]+$");
    private static final String INVALID_REQUEST = "invalid_request";
    private static final String INVALID_CODE = "invalid_code";

    private final OTPPolicy policy;

    public OtpRequestCheck(OTPPolicy policy) {
        this.policy = policy;
    }

    public byte[] checkedSecret(OtpRequest request) {
        requireRequest(request != null && request.secret != null && BASE32_SECRET.matcher(request.secret).matches());
        requireRequest(request.digits == policy.getDigits() && request.period == policy.getPeriod());
        requireRequest(policy.getAlgorithm().equals(request.algorithm));
        requireRequest(request.code != null && request.code.length() == policy.getDigits() && DIGITS_ONLY.matcher(request.code).matches());
        return Base32.decode(request.secret);
    }

    public void checkCode(OtpRequest request, byte[] secret) {
        TimeBasedOTP totp = new TimeBasedOTP(policy.getAlgorithm(), policy.getDigits(), policy.getPeriod(), policy.getLookAheadWindow());
        refuseUnless(totp.validateTOTP(request.code, secret), INVALID_CODE);
    }

    public long codeLifetimeSeconds() {
        return (long) policy.getPeriod() * (2L * policy.getLookAheadWindow() + 1L);
    }

    private void requireRequest(boolean condition) {
        refuseUnless(condition, INVALID_REQUEST);
    }

    private void refuseUnless(boolean condition, String error) {
        if (!condition) {
            throw new OtpRefused(Response.Status.BAD_REQUEST, error);
        }
    }
}
