package org.mesail.keycloak.otp;

public class OtpRequest {

    public String secret;
    public int digits;
    public int period;
    public String algorithm;
    public String code;
}
