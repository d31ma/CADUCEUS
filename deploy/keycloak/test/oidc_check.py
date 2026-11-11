import base64
import hashlib
import hmac
import struct
import time
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
from email.message import Message
from typing import Callable, NamedTuple


class OidcCheck:
    DEFAULT_DOMAINS = "example.com,example.org,partner.test"
    OPERATORS_REALM = "mesail-operators"
    IDENTITY_CLIENT = "mesail-identity"
    SERVICE_CLIENT = "mesail-service"
    ACCOUNT_CLIENT = "mesail-account"
    TEMPORARY_PASSWORD = "temporary-password-1"
    FAILURES_BEFORE_LOCK = 10
    QUICK_FAILURE_GAP_SECONDS = 1.5
    API_CLIENT = "mesail-api"
    MEMBER_ROLE = "mesail-member"
    ADMIN_ROLE = "mesail-admin"
    SERVICE_ROLE = "mesail-service"
    OPERATOR_ROLE = "mesail-operator"
    MEMBER_PASSWORD = "dev-member-password"
    ADMIN_PASSWORD = "dev-admin-password"
    OPERATOR_USERNAME = "operator@mesail.test"
    OPERATOR_PASSWORD = "dev-operator-password"
    TOKEN_LIFETIME = 60
    CALLBACK = "http://localhost/cb"
    LOGIN_FORM = 'id="kc-form-login"'
    SHOWN_CLAIMS = ["iss", "sub", "aud", "azp", "typ", "realm_access", "preferred_username", "exp", "iat"]

    def __init__(self, base_url: str, identity_secret: str, service_secret: str, account_secret: str, domains: list[str]) -> None:
        self.base_url = base_url.rstrip("/")
        self.identity_secret = identity_secret
        self.service_secret = service_secret
        self.account_secret = account_secret
        self.domains = domains
        self.http = Http()
        self.verifier = TokenVerifier(self.http)
        self.failures: list[str] = []

    def run(self, directory: "RealmDirectory | None") -> int:
        for index, domain in enumerate(self.domains):
            self.guarded(domain, lambda: self.check_domain(RealmUrls(self.base_url, domain), self.isolation_realm(index), index == 0))
        self.guarded(self.OPERATORS_REALM, self.check_operators)
        if directory is not None:
            self.guarded(directory.domain, lambda: self.check_added_domain(directory))
        return self.report()

    def check_domain(self, urls: "RealmUrls", other_realm: str, show_claims: bool) -> None:
        self.check_discovery(urls)
        member = self.check_member_token(urls, show_claims)
        self.check_admin_roles(urls)
        self.check_wrong_password(urls)
        self.check_introspection(urls, member["access_token"])
        self.check_isolation(urls, RealmUrls(self.base_url, other_realm), member["access_token"])
        self.check_refresh_rotation(urls, member["refresh_token"])
        self.check_service_token(urls)
        self.check_no_browser_login(urls)
        self.check_account_settings(urls, other_realm)
        if show_claims:
            self.check_lockout(urls)

    def check_discovery(self, urls: "RealmUrls") -> None:
        discovery = self.http.send(urls.discovery).json()
        self.expect(f"{urls.realm}: discovery issuer", discovery.get("issuer"), urls.issuer)

    def check_member_token(self, urls: "RealmUrls", show_claims: bool) -> dict:
        username = f"member@{urls.realm}"
        token = self.password_grant(urls, username, self.MEMBER_PASSWORD)
        claims = self.check_signature(urls, token["access_token"])
        self.check_identity_claims(urls, claims, username)
        if show_claims:
            print(f"{urls.realm}: member token claims:", json.dumps({name: claims.get(name) for name in self.SHOWN_CLAIMS}))
        return token

    def check_signature(self, urls: "RealmUrls", access_token: str) -> dict:
        algorithm, valid, claims = self.verifier.verify(urls.jwks, access_token)
        self.expect(f"{urls.realm}: token algorithm", algorithm, "RS256")
        self.expect(f"{urls.realm}: JWKS signature valid", valid, True)
        return claims

    def check_identity_claims(self, urls: "RealmUrls", claims: dict, username: str) -> None:
        realm = urls.realm
        self.expect(f"{realm}: iss", claims.get("iss"), urls.issuer)
        self.expect(f"{realm}: aud contains mesail-api", self.API_CLIENT in self.as_list(claims.get("aud")), True)
        self.expect(f"{realm}: azp", claims.get("azp"), self.IDENTITY_CLIENT)
        self.expect(f"{realm}: typ", claims.get("typ"), "Bearer")
        self.expect(f"{realm}: role mesail-member", self.MEMBER_ROLE in self.roles(claims), True)
        self.expect(f"{realm}: preferred_username", claims.get("preferred_username"), username)
        self.expect(f"{realm}: access token lifetime (s)", claims.get("exp", 0) - claims.get("iat", 0), self.TOKEN_LIFETIME)
        self.expect(f"{realm}: sub present", bool(claims.get("sub")), True)

    def check_admin_roles(self, urls: "RealmUrls") -> None:
        token = self.password_grant(urls, f"admin@{urls.realm}", self.ADMIN_PASSWORD)
        roles = self.roles(self.verifier.claims(token["access_token"]))
        self.expect(f"{urls.realm}: admin role mesail-admin", self.ADMIN_ROLE in roles, True)
        self.expect(f"{urls.realm}: admin role mesail-member", self.MEMBER_ROLE in roles, True)

    def check_wrong_password(self, urls: "RealmUrls") -> None:
        reply = self.password_reply(urls, f"member@{urls.realm}", f"wrong-{self.MEMBER_PASSWORD}")
        self.expect(f"{urls.realm}: wrong password status", reply.status, 400)
        self.expect(f"{urls.realm}: wrong password error", reply.json().get("error"), "invalid_grant")

    def check_introspection(self, urls: "RealmUrls", access_token: str) -> None:
        self.expect(f"{urls.realm}: introspection of member token: active", self.introspect(urls, access_token), True)
        self.expect(f"{urls.realm}: introspection of a tampered token: active", self.introspect(urls, self.tampered(access_token)), False)

    def check_isolation(self, urls: "RealmUrls", other: "RealmUrls", access_token: str) -> None:
        self.expect(f"{urls.realm}: member token introspected at {other.realm}: active", self.introspect(other, access_token), False)

    def check_refresh_rotation(self, urls: "RealmUrls", refresh_token: str) -> None:
        first = self.refresh_reply(urls, refresh_token)
        self.expect(f"{urls.realm}: first refresh status", first.status, 200)
        self.expect(f"{urls.realm}: first refresh rotated the refresh token", first.json().get("refresh_token") not in (None, refresh_token), True)
        second = self.refresh_reply(urls, refresh_token)
        self.expect(f"{urls.realm}: reused refresh token status", second.status, 400)
        self.expect(f"{urls.realm}: reused refresh token error", second.json().get("error"), "invalid_grant")

    def check_service_token(self, urls: "RealmUrls") -> None:
        reply = self.http.post_form(urls.token, {"grant_type": "client_credentials"}, (self.SERVICE_CLIENT, self.service_secret))
        claims = self.verifier.claims(self.require_ok(reply, f"{urls.realm}: service token")["access_token"])
        self.expect(f"{urls.realm}: service token azp", claims.get("azp"), self.SERVICE_CLIENT)
        self.expect(f"{urls.realm}: service token aud contains mesail-api", self.API_CLIENT in self.as_list(claims.get("aud")), True)
        self.expect(f"{urls.realm}: service token role mesail-service", self.SERVICE_ROLE in self.roles(claims), True)
        self.expect(f"{urls.realm}: service token has no member or operator role",
                    sorted(set(self.roles(claims)) & {self.MEMBER_ROLE, self.OPERATOR_ROLE}), [])

    def check_no_browser_login(self, urls: "RealmUrls") -> None:
        query = urllib.parse.urlencode({
            "client_id": self.IDENTITY_CLIENT, "response_type": "code", "scope": "openid", "redirect_uri": self.CALLBACK,
        })
        outcome, passed = self.browser_outcome(self.http.send(f"{urls.auth}?{query}"))
        self.expect(f"{urls.realm}: no browser login form offered ({outcome})", passed, True)

    def check_operators(self) -> None:
        urls = RealmUrls(self.base_url, self.OPERATORS_REALM)
        token = self.password_grant(urls, self.OPERATOR_USERNAME, self.OPERATOR_PASSWORD)
        claims = self.check_signature(urls, token["access_token"])
        self.expect(f"{urls.realm}: role mesail-operator", self.OPERATOR_ROLE in self.roles(claims), True)
        self.expect(f"{urls.realm}: azp", claims.get("azp"), self.IDENTITY_CLIENT)
        self.check_service_token(urls)

    def check_account_settings(self, urls: "RealmUrls", other_realm: str) -> None:
        account = AccountSettings(self.http, self.base_url, urls.realm, self.account_secret)
        realm = urls.realm
        username = f"member@{realm}"
        self.expect(f"{realm}: mesail-account token (HTTP)", account.sign_in(), 200)
        user_id = account.user_id(username)
        self.expect(f"{realm}: mesail-account finds the user", bool(user_id), True)
        secret = base64.b32encode(os.urandom(20)).decode().rstrip("=")
        self.expect(f"{realm}: store a TOTP with a given base32 secret (HTTP)", account.store_totp(user_id, secret), 204)
        time.sleep(self.QUICK_FAILURE_GAP_SECONDS)
        self.expect(f"{realm}: sign-in without the TOTP code (HTTP)", self.password_reply(urls, username, self.MEMBER_PASSWORD).status, 400)
        self.expect(f"{realm}: sign-in with the TOTP code (HTTP)", self.password_reply(urls, username, self.MEMBER_PASSWORD, self.totp(secret)).status, 200)
        self.expect(f"{realm}: delete the TOTP credential (HTTP)", account.delete_credentials(user_id, "otp"), 204)
        self.expect(f"{realm}: sign-in without TOTP after removal (HTTP)", self.password_reply(urls, username, self.MEMBER_PASSWORD).status, 200)
        self.expect(f"{realm}: reset the password (HTTP)", account.reset_password(user_id, self.TEMPORARY_PASSWORD), 204)
        self.expect(f"{realm}: sign-in with the new password (HTTP)", self.password_reply(urls, username, self.TEMPORARY_PASSWORD).status, 200)
        self.expect(f"{realm}: set the password back (HTTP)", account.reset_password(user_id, self.MEMBER_PASSWORD), 204)
        refresh_token = self.password_grant(urls, username, self.MEMBER_PASSWORD)["refresh_token"]
        self.expect(f"{realm}: log the user out everywhere (HTTP)", account.logout(user_id), 204)
        self.expect(f"{realm}: refresh after the logout (HTTP)", self.refresh_reply(urls, refresh_token).status, 400)
        self.expect(f"{realm}: mesail-account may not list clients (HTTP)", account.get(f"/{realm}/clients"), 403)
        self.expect(f"{realm}: mesail-account may not read another realm (HTTP)", account.get(f"/{other_realm}/users"), 403)

    def check_lockout(self, urls: "RealmUrls") -> None:
        realm = urls.realm
        username = f"admin@{realm}"
        for _ in range(self.FAILURES_BEFORE_LOCK):
            self.password_reply(urls, username, "wrong-password-0")
        locked = self.password_reply(urls, username, self.ADMIN_PASSWORD)
        self.expect(f"{realm}: locked account with the right password (HTTP, error, description)",
                    (locked.status, locked.json().get("error"), locked.json().get("error_description")), (400, "invalid_grant", "Invalid user credentials"))
        account = AccountSettings(self.http, self.base_url, realm, self.account_secret)
        account.sign_in()
        user_id = account.user_id(username)
        self.expect(f"{realm}: lock status says disabled", account.lock_status(user_id).get("disabled"), True)
        self.expect(f"{realm}: clear the lock (HTTP)", account.clear_lock(user_id), 204)
        self.expect(f"{realm}: sign-in after the lock is cleared (HTTP)", self.password_reply(urls, username, self.ADMIN_PASSWORD).status, 200)

    def check_added_domain(self, directory: "RealmDirectory") -> None:
        realm = directory.domain
        self.expect(f"{realm}: directory client token from master (HTTP)", directory.sign_in(), 200)
        status = directory.realm_status()
        self.expect(f"{realm}: realm lookup (HTTP {status})", status in (200, 404), True)
        if status == 404:
            self.expect(f"{realm}: realm created (HTTP)", directory.create_realm(), 201)
            self.expect(f"{realm}: new directory token after creating the realm (HTTP)", directory.sign_in(), 200)
        else:
            print(f"{realm}: realm already exists; skipping creation")
        status = directory.create_member()
        self.expect(f"{realm}: member user created or present (HTTP {status})", status in (201, 409), True)
        urls = RealmUrls(self.base_url, realm)
        self.check_discovery(urls)
        member = self.check_member_token(urls, False)
        self.check_introspection(urls, member["access_token"])
        self.check_account_settings(urls, self.OPERATORS_REALM)

    def browser_outcome(self, reply: "Reply") -> tuple[str, bool]:
        if 300 <= reply.status < 400:
            location = reply.headers.get("Location", "")
            return f"redirect to {location}", "error=unauthorized_client" in location
        if reply.status == 200 or 400 <= reply.status < 500:
            offered = self.LOGIN_FORM in reply.text()
            return f"HTTP {reply.status}, login form {'present' if offered else 'absent'}", not offered
        return f"HTTP {reply.status}", False

    def password_grant(self, urls: "RealmUrls", username: str, password: str) -> dict:
        return self.require_ok(self.password_reply(urls, username, password), f"{urls.realm}: password grant for {username}")

    def password_reply(self, urls: "RealmUrls", username: str, password: str, totp: str | None = None) -> "Reply":
        fields = {
            "grant_type": "password", "client_id": self.IDENTITY_CLIENT, "client_secret": self.identity_secret,
            "username": username, "password": password, "scope": "openid",
        }
        if totp is not None:
            fields["totp"] = totp
        return self.http.post_form(urls.token, fields)

    @staticmethod
    def totp(secret: str) -> str:
        key = base64.b32decode(secret + "=" * (-len(secret) % 8))
        digest = hmac.new(key, struct.pack(">Q", int(time.time()) // 30), hashlib.sha1).digest()
        offset = digest[-1] & 15
        return str((struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF) % 1000000).zfill(6)

    def refresh_reply(self, urls: "RealmUrls", refresh_token: str) -> "Reply":
        return self.http.post_form(urls.token, {
            "grant_type": "refresh_token", "client_id": self.IDENTITY_CLIENT, "client_secret": self.identity_secret,
            "refresh_token": refresh_token,
        })

    def introspect(self, urls: "RealmUrls", token: str) -> object:
        reply = self.http.post_form(urls.introspection, {"token": token}, (self.IDENTITY_CLIENT, self.identity_secret))
        return reply.json().get("active", f"HTTP {reply.status}")

    def isolation_realm(self, index: int) -> str:
        if len(self.domains) < 2:
            return self.OPERATORS_REALM
        return self.domains[(index + 1) % len(self.domains)]

    def guarded(self, name: str, action: Callable[[], None]) -> None:
        try:
            action()
        except Exception as error:
            self.expect(f"{name}: checks ran without error", f"{type(error).__name__}: {error}", None)

    def expect(self, name: str, actual: object, expected: object) -> None:
        passed = actual == expected
        print(f"{'PASS' if passed else 'FAIL'} {name}: {actual!r}")
        if not passed:
            self.failures.append(f"{name}: expected {expected!r}, got {actual!r}")

    def report(self) -> int:
        print(f"=== oidc check: {'all passed' if not self.failures else str(len(self.failures)) + ' failed'}")
        return 1 if self.failures else 0

    @staticmethod
    def require_ok(reply: "Reply", action: str) -> dict:
        if reply.status != 200:
            raise AssertionError(f"{action} failed: HTTP {reply.status} {reply.text()[:300]}")
        return reply.json()

    @staticmethod
    def tampered(token: str) -> str:
        return token[:-4] + ("AAAA" if not token.endswith("AAAA") else "BBBB")

    @staticmethod
    def roles(claims: dict) -> list:
        return claims.get("realm_access", {}).get("roles", [])

    @staticmethod
    def as_list(value: object) -> list:
        return value if isinstance(value, list) else [value]

    @staticmethod
    def domains_from(text: str) -> list[str]:
        return [domain.strip() for domain in text.split(",") if domain.strip()]


class RealmUrls:
    def __init__(self, base_url: str, realm: str) -> None:
        self.realm = realm
        self.issuer = f"{base_url}/realms/{realm}"
        self.discovery = f"{self.issuer}/.well-known/openid-configuration"
        connect = f"{self.issuer}/protocol/openid-connect"
        self.token = f"{connect}/token"
        self.introspection = f"{connect}/token/introspect"
        self.jwks = f"{connect}/certs"
        self.auth = f"{connect}/auth"


class AccountSettings:
    CLIENT = "mesail-account"

    def __init__(self, http: "Http", base_url: str, realm: str, secret: str) -> None:
        self.http = http
        self.base_url = base_url
        self.realm = realm
        self.secret = secret
        self.token = ""

    def sign_in(self) -> int:
        reply = self.http.post_form(f"{self.base_url}/realms/{self.realm}/protocol/openid-connect/token",
                                    {"grant_type": "client_credentials"}, (self.CLIENT, self.secret))
        self.token = reply.json().get("access_token", "")
        return reply.status

    def user_id(self, username: str) -> str:
        reply = self.call("GET", f"/{self.realm}/users?" + urllib.parse.urlencode({"username": username, "exact": "true"}))
        users = reply.json_list()
        return users[0]["id"] if users else ""

    def store_totp(self, user_id: str, secret: str) -> int:
        credential = {
            "type": "otp", "userLabel": "Mesail",
            "secretData": json.dumps({"value": secret}),
            "credentialData": json.dumps({"subType": "totp", "digits": 6, "period": 30, "algorithm": "HmacSHA1", "counter": 0, "secretEncoding": "BASE32"}),
        }
        return self.call("PUT", f"/{self.realm}/users/{user_id}", {"credentials": [credential]}).status

    def delete_credentials(self, user_id: str, credential_type: str) -> int:
        credentials = self.call("GET", f"/{self.realm}/users/{user_id}/credentials").json_list()
        statuses = [self.call("DELETE", f"/{self.realm}/users/{user_id}/credentials/{credential['id']}").status
                    for credential in credentials if credential["type"] == credential_type]
        return statuses[0] if statuses else 404

    def reset_password(self, user_id: str, password: str) -> int:
        return self.call("PUT", f"/{self.realm}/users/{user_id}/reset-password", {"type": "password", "value": password, "temporary": False}).status

    def logout(self, user_id: str) -> int:
        return self.call("POST", f"/{self.realm}/users/{user_id}/logout").status

    def lock_status(self, user_id: str) -> dict:
        return self.call("GET", f"/{self.realm}/attack-detection/brute-force/users/{user_id}").json()

    def clear_lock(self, user_id: str) -> int:
        return self.call("DELETE", f"/{self.realm}/attack-detection/brute-force/users/{user_id}").status

    def get(self, path: str) -> int:
        return self.call("GET", path).status

    def call(self, method: str, path: str, body: object = None) -> "Reply":
        headers = {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}
        data = None if body is None else json.dumps(body).encode()
        return self.http.send(f"{self.base_url}/admin/realms{path}", data, headers, method)


class RealmDirectory:
    MEMBER_PASSWORD = "dev-member-password"
    PLACEHOLDER = re.compile(r"\$\{([A-Za-z0-9_]+)\}")
    SETTINGS = ["CHECK_ADD_DOMAIN", "MESAIL_KEYCLOAK_DIRECTORY_CLIENT_ID", "MESAIL_KEYCLOAK_DIRECTORY_SECRET", "CHECK_REALM_TEMPLATE"]

    def __init__(self, base_url: str, domain: str, client_id: str, secret: str, template_path: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.domain = domain
        self.client_id = client_id
        self.secret = secret
        self.template_path = template_path
        self.http = Http()
        self.token = ""

    @classmethod
    def from_environment(cls, base_url: str) -> "RealmDirectory | None":
        if not os.environ.get("CHECK_ADD_DOMAIN"):
            return None
        missing = [name for name in cls.SETTINGS if not os.environ.get(name)]
        if missing:
            raise SystemExit(f"CHECK_ADD_DOMAIN also needs {', '.join(missing)}")
        return cls(base_url, *[os.environ[name] for name in cls.SETTINGS])

    def sign_in(self) -> int:
        url = f"{self.base_url}/realms/master/protocol/openid-connect/token"
        reply = self.http.post_form(url, {"grant_type": "client_credentials"}, (self.client_id, self.secret))
        self.token = reply.json().get("access_token", "")
        return reply.status

    def realm_status(self) -> int:
        return self.http.send(f"{self.base_url}/realms/{self.domain}/.well-known/openid-configuration").status

    def create_realm(self) -> int:
        return self.admin("POST", "", self.rendered_template().encode()).status

    def create_member(self) -> int:
        username = f"member@{self.domain}"
        user = {
            "username": username, "email": username, "firstName": "Dev", "lastName": "Member",
            "enabled": True, "emailVerified": True,
            "credentials": [{"type": "password", "value": self.MEMBER_PASSWORD, "temporary": False}],
        }
        return self.admin("POST", f"/{self.domain}/users", json.dumps(user).encode()).status

    def rendered_template(self) -> str:
        values = dict(os.environ, MESAIL_REALM=self.domain)
        with open(self.template_path, encoding="utf-8") as handle:
            text = handle.read()
        return self.PLACEHOLDER.sub(lambda match: json.dumps(values[match.group(1)])[1:-1], text)

    def admin(self, method: str, path: str, body: bytes | None = None) -> "Reply":
        headers = {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}
        return self.http.send(f"{self.base_url}/admin/realms{path}", body, headers, method)


class TokenVerifier:
    def __init__(self, http: "Http") -> None:
        self.http = http

    def verify(self, jwks_url: str, token: str) -> tuple[object, bool, dict]:
        header_part, payload_part, signature_part = token.split(".")
        header = json.loads(self.unbase64(header_part))
        keys = self.http.send(jwks_url).json().get("keys", [])
        key = next((candidate for candidate in keys if candidate.get("kid") == header.get("kid")), None)
        valid = key is not None and self.signature_valid(key, f"{header_part}.{payload_part}", self.unbase64(signature_part))
        return header.get("alg"), valid, json.loads(self.unbase64(payload_part))

    def claims(self, token: str) -> dict:
        return json.loads(self.unbase64(token.split(".")[1]))

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

    @staticmethod
    def unbase64(text: str) -> bytes:
        return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))

    @staticmethod
    def write(path: str, content: bytes) -> None:
        with open(path, "wb") as handle:
            handle.write(content)


class Reply(NamedTuple):
    status: int
    headers: Message
    body: bytes

    def text(self) -> str:
        return self.body.decode(errors="replace")

    def json(self) -> dict:
        try:
            value = json.loads(self.body)
        except ValueError:
            return {}
        return value if isinstance(value, dict) else {}

    def json_list(self) -> list:
        try:
            value = json.loads(self.body)
        except ValueError:
            return []
        return value if isinstance(value, list) else []


class Http:
    def __init__(self) -> None:
        self.opener = urllib.request.build_opener(NoRedirect())

    def send(self, url: str, data: bytes | None = None, headers: dict[str, str] | None = None, method: str | None = None) -> Reply:
        request = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
        try:
            with self.opener.open(request) as response:
                return Reply(response.status, response.headers, response.read())
        except urllib.error.HTTPError as error:
            return Reply(error.code, error.headers, error.read())

    def post_form(self, url: str, fields: dict[str, str], client: tuple[str, str] | None = None) -> Reply:
        headers = {"Content-Type": "application/x-www-form-urlencoded"}
        if client is not None:
            headers["Authorization"] = "Basic " + base64.b64encode(f"{client[0]}:{client[1]}".encode()).decode()
        return self.send(url, urllib.parse.urlencode(fields).encode(), headers)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


if __name__ == "__main__":
    keycloak_url = os.environ["MESAIL_KEYCLOAK_URL"]
    sys.exit(OidcCheck(
        keycloak_url, os.environ["MESAIL_IDENTITY_CLIENT_SECRET"], os.environ["MESAIL_SERVICE_SECRET"], os.environ["MESAIL_ACCOUNT_CLIENT_SECRET"],
        OidcCheck.domains_from(os.environ.get("CHECK_DOMAINS", OidcCheck.DEFAULT_DOMAINS)),
    ).run(RealmDirectory.from_environment(keycloak_url)))
