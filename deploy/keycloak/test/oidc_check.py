import base64
import hashlib
import html
import http.cookiejar
import json
import os
import re
import secrets
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request


class OidcCheck:
    REALM = "mesail"
    WEB_CLIENT = "mesail-web"
    API_CLIENT = "mesail-api"
    SERVICE_CLIENT = "mesail-service"
    FORM_ACTION = re.compile(r'<form[^>]*id="kc-form-login"[^>]*action="([^"]+)"')

    def __init__(self, public_url: str, api_secret: str, service_secret: str, username: str, password: str) -> None:
        self.public_url = public_url.rstrip("/")
        self.api_secret = api_secret
        self.service_secret = service_secret
        self.username = username
        self.password = password
        self.issuer = f"{self.public_url}/idp/realms/{self.REALM}"
        self.cookies = http.cookiejar.CookieJar(LocalhostIsSecure())
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(self.cookies), NoRedirect())
        self.failures: list[str] = []

    def run(self) -> int:
        discovery = self.check_discovery()
        token = self.check_pkce_login(discovery)
        claims = self.check_signature(discovery, token["access_token"])
        self.check_user_claims(claims)
        self.check_introspection(discovery, token["access_token"])
        self.check_service_token(discovery)
        self.check_bad_token_rejected(discovery, token["access_token"])
        return self.report()

    def check_discovery(self) -> dict:
        discovery = self.get_json(f"{self.issuer}/.well-known/openid-configuration")
        self.expect("discovery issuer", discovery["issuer"], self.issuer)
        self.expect("PKCE S256 offered", "S256" in discovery["code_challenge_methods_supported"], True)
        return discovery

    def check_pkce_login(self, discovery: dict) -> dict:
        verifier = secrets.token_urlsafe(48)
        challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
        redirect_uri = f"{self.public_url}/callback"
        query = urllib.parse.urlencode({
            "client_id": self.WEB_CLIENT, "response_type": "code", "scope": "openid email profile",
            "redirect_uri": redirect_uri, "state": secrets.token_hex(8), "code_challenge": challenge,
            "code_challenge_method": "S256",
        })
        login_page = self.opener.open(f"{discovery['authorization_endpoint']}?{query}").read().decode()
        action = html.unescape(self.FORM_ACTION.search(login_page).group(1))
        form = urllib.parse.urlencode({"username": self.username, "password": self.password}).encode()
        location = self.redirect_location(urllib.request.Request(action, data=form))
        code = self.authorization_code(location)
        token = self.post_form(discovery["token_endpoint"], {
            "grant_type": "authorization_code", "client_id": self.WEB_CLIENT, "code": code,
            "redirect_uri": redirect_uri, "code_verifier": verifier,
        })
        self.expect("PKCE login returned an access token", "access_token" in token, True)
        return token

    def check_signature(self, discovery: dict, access_token: str) -> dict:
        header_part, payload_part, signature_part = access_token.split(".")
        header = json.loads(self.unbase64(header_part))
        keys = self.get_json(discovery["jwks_uri"])["keys"]
        key = next(candidate for candidate in keys if candidate["kid"] == header["kid"])
        self.expect("token algorithm", header["alg"], "RS256")
        self.expect("JWKS signature valid", self.signature_valid(key, f"{header_part}.{payload_part}", self.unbase64(signature_part)), True)
        return json.loads(self.unbase64(payload_part))

    def check_user_claims(self, claims: dict) -> None:
        self.expect("iss", claims["iss"], self.issuer)
        self.expect("aud contains mesail-api", self.API_CLIENT in self.as_list(claims["aud"]), True)
        self.expect("azp", claims["azp"], self.WEB_CLIENT)
        self.expect("role mesail-member", "mesail-member" in claims["realm_access"]["roles"], True)
        self.expect("preferred_username", claims["preferred_username"], self.username)
        self.expect("access token lifetime (s)", claims["exp"] - claims["iat"], 60)
        print("user token claims:", json.dumps({name: claims.get(name) for name in
              ["iss", "sub", "aud", "azp", "realm_access", "preferred_username", "email", "name", "exp", "iat"]}))

    def check_introspection(self, discovery: dict, access_token: str) -> None:
        result = self.post_form(discovery["introspection_endpoint"], {"token": access_token}, (self.API_CLIENT, self.api_secret))
        self.expect("introspection by mesail-api: active", result.get("active"), True)

    def check_bad_token_rejected(self, discovery: dict, access_token: str) -> None:
        tampered = access_token[:-4] + ("AAAA" if not access_token.endswith("AAAA") else "BBBB")
        result = self.post_form(discovery["introspection_endpoint"], {"token": tampered}, (self.API_CLIENT, self.api_secret))
        self.expect("introspection of a tampered token: active", result.get("active"), False)

    def check_service_token(self, discovery: dict) -> None:
        token = self.post_form(discovery["token_endpoint"], {"grant_type": "client_credentials"}, (self.SERVICE_CLIENT, self.service_secret))
        claims = json.loads(self.unbase64(token["access_token"].split(".")[1]))
        self.expect("service token azp", claims["azp"], self.SERVICE_CLIENT)
        self.expect("service token aud contains mesail-api", self.API_CLIENT in self.as_list(claims["aud"]), True)
        self.expect("service token role mesail-service", "mesail-service" in claims["realm_access"]["roles"], True)

    def signature_valid(self, key: dict, signed_text: str, signature: bytes) -> bool:
        with tempfile.TemporaryDirectory() as folder:
            paths = {name: os.path.join(folder, name) for name in ["key.pem", "data", "signature"]}
            self.write(paths["key.pem"], self.rsa_public_key_pem(key).encode())
            self.write(paths["data"], signed_text.encode())
            self.write(paths["signature"], signature)
            command = ["openssl", "dgst", "-sha256", "-verify", paths["key.pem"], "-signature", paths["signature"], paths["data"]]
            return subprocess.run(command, capture_output=True).returncode == 0

    def rsa_public_key_pem(self, key: dict) -> str:
        modulus = self.der_integer(self.unbase64(key["n"]))
        exponent = self.der_integer(self.unbase64(key["e"]))
        rsa_key = self.der(0x30, modulus + exponent)
        algorithm = self.der(0x30, bytes.fromhex("06092a864886f70d0101010500"))
        public_key_info = self.der(0x30, algorithm + self.der(0x03, b"\x00" + rsa_key))
        body = base64.encodebytes(public_key_info).decode()
        return f"-----BEGIN PUBLIC KEY-----\n{body}-----END PUBLIC KEY-----\n"

    def der_integer(self, value: bytes) -> bytes:
        return self.der(0x02, b"\x00" + value if value[0] & 0x80 else value)

    def der(self, tag: int, content: bytes) -> bytes:
        size = len(content)
        if size < 0x80:
            return bytes([tag, size]) + content
        size_bytes = size.to_bytes((size.bit_length() + 7) // 8, "big")
        return bytes([tag, 0x80 | len(size_bytes)]) + size_bytes + content

    def authorization_code(self, location: str) -> str:
        codes = urllib.parse.parse_qs(urllib.parse.urlparse(location).query).get("code")
        if not codes:
            raise AssertionError(f"login redirected without a code: {location}")
        return codes[0]

    def redirect_location(self, request: urllib.request.Request) -> str:
        try:
            response = self.opener.open(request)
        except urllib.error.HTTPError as redirect:
            return redirect.headers["Location"]
        raise AssertionError(f"login did not redirect (status {response.status}); wrong password or required action?")

    def post_form(self, url: str, fields: dict, client: tuple[str, str] | None = None) -> dict:
        request = urllib.request.Request(url, data=urllib.parse.urlencode(fields).encode())
        if client is not None:
            credentials = base64.b64encode(f"{client[0]}:{client[1]}".encode()).decode()
            request.add_header("Authorization", f"Basic {credentials}")
        return json.loads(urllib.request.urlopen(request).read())

    def get_json(self, url: str) -> dict:
        return json.loads(urllib.request.urlopen(url).read())

    def expect(self, name: str, actual: object, expected: object) -> None:
        passed = actual == expected
        print(f"{'PASS' if passed else 'FAIL'} {name}: {actual!r}")
        if not passed:
            self.failures.append(f"{name}: expected {expected!r}, got {actual!r}")

    def report(self) -> int:
        print(f"=== oidc check: {'all passed' if not self.failures else str(len(self.failures)) + ' failed'}")
        return 1 if self.failures else 0

    @staticmethod
    def as_list(value: object) -> list:
        return value if isinstance(value, list) else [value]

    @staticmethod
    def unbase64(text: str) -> bytes:
        return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))

    @staticmethod
    def write(path: str, content: bytes) -> None:
        with open(path, "wb") as handle:
            handle.write(content)


class LocalhostIsSecure(http.cookiejar.DefaultCookiePolicy):
    LOCAL_HOSTS = ("localhost", "127.0.0.1")

    def return_ok_secure(self, cookie, request):
        return urllib.parse.urlparse(request.get_full_url()).hostname in self.LOCAL_HOSTS or super().return_ok_secure(cookie, request)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


if __name__ == "__main__":
    sys.exit(OidcCheck(
        os.environ["MESAIL_PUBLIC_URL"], os.environ["MESAIL_KEYCLOAK_API_SECRET"], os.environ["MESAIL_KEYCLOAK_SERVICE_SECRET"],
        os.environ.get("CHECK_USERNAME", "member@mesail.test"), os.environ.get("CHECK_PASSWORD", "dev-member-password"),
    ).run())
