import json
import os
import sys
import time
import urllib.parse
from typing import Callable

from keycloak_client import AccountSettings, Http, Reply, Totp, TokenVerifier, account_status


class MesailRealmCheck:
    REALM = "mesail"
    IDENTITY_CLIENT = "mesail-identity"
    ACCOUNT_CLIENT = "mesail-account"
    DIRECTORY_CLIENT = "mesail-directory"
    REMOVED_CLIENTS = ["mesail-service", "mesail-api"]
    OPERATOR_ROLE = "mesail-operator"
    OLD_ROLES = ["mesail-member", "mesail-admin", "mesail-service"]
    MUST_CHANGE_ATTRIBUTE = "mesail.mustChangePassword"
    MUST_CHANGE_CLAIM = "mesail_must_change_password"
    PEOPLE = {
        "ana": "dev-ana-password",
        "bo": "dev-bo-password",
        "eve": "dev-eve-password",
        "carol": "dev-carol-password",
        "operator": "dev-operator-password",
    }
    OTP_PERSON = "bo"
    LOCK_PERSON = "eve"
    NEW_PERSON = "dan.new-person_1"
    NEW_PASSWORD = "first-password-1"
    TEMPORARY_PASSWORD = "temporary-password-1"
    FAILURES_BEFORE_LOCK = 10
    QUICK_FAILURE_GAP_SECONDS = 1.5
    FRESH_STEP_MARGIN_SECONDS = 20
    TOKEN_LIFETIME = 60
    CALLBACK = "http://localhost/cb"
    LOGIN_FORM = 'id="kc-form-login"'
    SHOWN_CLAIMS = ["iss", "sub", "aud", "azp", "typ", "realm_access", "preferred_username", "exp", "iat"]

    def __init__(self, base_url: str, identity_secret: str, account_secret: str, directory_secret: str, legacy_realms: bool) -> None:
        self.base_url = base_url.rstrip("/")
        self.legacy_realms = legacy_realms
        self.identity_secret = identity_secret
        self.account_secret = account_secret
        self.directory_secret = directory_secret
        self.http = Http()
        self.verifier = TokenVerifier(self.http)
        self.urls = RealmUrls(self.base_url, self.REALM)
        self.failures: list[str] = []

    def run(self) -> int:
        self.guarded("discovery", self.check_discovery)
        self.guarded("people", self.check_people)
        self.guarded("username", self.check_username_rules)
        self.guarded("tokens", self.check_token_handling)
        self.guarded("clients", self.check_clients)
        self.guarded("directory", self.check_directory)
        self.guarded("account", self.check_account_settings)
        self.guarded("otp", self.check_otp)
        self.guarded("lockout", self.check_lockout)
        return self.report()

    def check_discovery(self) -> None:
        discovery = self.http.send(self.urls.discovery).json()
        self.expect("discovery issuer", discovery.get("issuer"), self.urls.issuer)
        if self.legacy_realms:
            self.expect("old realm mesail-operators still answers (transition, until L5)", self.discovery_status("mesail-operators"), 200)
            return
        self.expect("no other realm: mesail-operators answers 404", self.discovery_status("mesail-operators"), 404)

    def check_people(self) -> None:
        for username, password in self.PEOPLE.items():
            token = self.password_grant(username, password)
            claims = self.check_signature(username, token["access_token"])
            self.check_person_claims(username, claims)
        operator = self.verifier.claims(self.password_grant("operator", self.PEOPLE["operator"])["access_token"])
        print("operator token claims:", json.dumps({name: operator.get(name) for name in self.SHOWN_CLAIMS}))
        self.expect("operator: role mesail-operator", self.OPERATOR_ROLE in self.roles(operator), True)

    def check_signature(self, username: str, access_token: str) -> dict:
        algorithm, valid, claims = self.verifier.verify(self.urls.jwks, access_token)
        self.expect(f"{username}: token algorithm", algorithm, "RS256")
        self.expect(f"{username}: JWKS signature valid", valid, True)
        return claims

    def check_person_claims(self, username: str, claims: dict) -> None:
        self.expect(f"{username}: iss", claims.get("iss"), self.urls.issuer)
        self.expect(f"{username}: azp", claims.get("azp"), self.IDENTITY_CLIENT)
        self.expect(f"{username}: aud is only mesail-identity", self.as_list(claims.get("aud")), [self.IDENTITY_CLIENT])
        self.expect(f"{username}: typ", claims.get("typ"), "Bearer")
        self.expect(f"{username}: preferred_username is the prefix", claims.get("preferred_username"), username)
        self.expect(f"{username}: access token lifetime (s)", claims.get("exp", 0) - claims.get("iat", 0), self.TOKEN_LIFETIME)
        self.expect(f"{username}: sid present", bool(claims.get("sid")), True)
        self.expect(f"{username}: no old domain roles", sorted(set(self.roles(claims)) & set(self.OLD_ROLES)), [])
        self.expect(f"{username}: mesail-operator only for the operator",
                    self.OPERATOR_ROLE in self.roles(claims), username == "operator")
        self.expect(f"{username}: no must-change claim", self.MUST_CHANGE_CLAIM in claims, False)

    def check_username_rules(self) -> None:
        ana = self.verifier.claims(self.password_grant("ana", self.PEOPLE["ana"])["access_token"])
        upper = self.verifier.claims(self.password_grant("ANA", self.PEOPLE["ana"])["access_token"])
        self.expect("ANA signs in as ana (preferred_username)", upper.get("preferred_username"), "ana")
        self.expect("ANA signs in as ana (same sub)", upper.get("sub"), ana.get("sub"))
        padded = self.verifier.claims(self.password_grant(" Ana ", self.PEOPLE["ana"])["access_token"])
        self.expect("' Ana ' with spaces signs in as ana (Keycloak trims)", padded.get("sub"), ana.get("sub"))
        by_email = self.password_reply("ana@example.com", self.PEOPLE["ana"])
        self.expect("full address refused by Keycloak; identity splits it (HTTP, error)",
                    (by_email.status, by_email.json().get("error")), (400, "invalid_grant"))
        wrong = self.password_reply("ana", f"wrong-{self.PEOPLE['ana']}")
        self.expect("wrong password (HTTP, error)", (wrong.status, wrong.json().get("error")), (400, "invalid_grant"))
        time.sleep(self.QUICK_FAILURE_GAP_SECONDS)

    def check_token_handling(self) -> None:
        token = self.password_grant("carol", self.PEOPLE["carol"])
        self.expect("introspection of carol's token: active", self.introspect(token["access_token"]), True)
        self.expect("introspection of a tampered token: active", self.introspect(self.tampered(token["access_token"])), False)
        first = self.refresh_reply(token["refresh_token"])
        self.expect("first refresh status", first.status, 200)
        self.expect("first refresh rotated the refresh token", first.json().get("refresh_token") not in (None, token["refresh_token"]), True)
        second = self.refresh_reply(token["refresh_token"])
        self.expect("reused refresh token (HTTP, error)", (second.status, second.json().get("error")), (400, "invalid_grant"))
        query = urllib.parse.urlencode({"client_id": self.IDENTITY_CLIENT, "response_type": "code", "scope": "openid", "redirect_uri": self.CALLBACK})
        outcome, passed = self.browser_outcome(self.http.send(f"{self.urls.auth}?{query}"))
        self.expect(f"no browser login form offered ({outcome})", passed, True)

    def check_clients(self) -> None:
        identity = self.client_credentials(self.IDENTITY_CLIENT, self.identity_secret)
        self.expect("mesail-identity has no client-credentials grant (HTTP, error)",
                    (identity.status, identity.json().get("error")), (401, "unauthorized_client"))
        for client in self.REMOVED_CLIENTS:
            reply = self.client_credentials(client, self.identity_secret)
            self.expect(f"client {client} does not exist (HTTP, error)", (reply.status, reply.json().get("error")), (401, "invalid_client"))
        for client, secret in [(self.ACCOUNT_CLIENT, self.account_secret), (self.DIRECTORY_CLIENT, self.directory_secret)]:
            claims = self.verifier.claims(self.require_ok(self.client_credentials(client, secret), f"{client} token")["access_token"])
            self.expect(f"{client}: service token has no realm role", self.roles(claims), [])
            self.expect(f"{client}: service token is not for mesail-identity", self.IDENTITY_CLIENT in self.as_list(claims.get("aud")), False)

    def check_directory(self) -> None:
        directory = self.admin_client(self.DIRECTORY_CLIENT, self.directory_secret)
        self.expect("directory: may not list clients (HTTP)", directory.get(f"/{self.REALM}/clients"), 403)
        self.expect("directory: may not read the master realm (HTTP)", directory.get("/master/users"), 403)
        self.expect("directory: may not create a realm (HTTP)", directory.call("POST", "", {"realm": "not-allowed.test", "enabled": True}).status, 403)
        self.expect("directory: query-users lists ana (HTTP)", self.usernames(directory, "ana"), ["ana"])
        self.remove_person_if_present(directory, self.NEW_PERSON)
        created = directory.call("POST", f"/{self.REALM}/users", self.new_person(self.NEW_PERSON.upper()))
        self.expect("directory: creates a person, username given in capitals (HTTP)", created.status, 201)
        self.expect("directory: the username is stored in lowercase", self.usernames(directory, self.NEW_PERSON), [self.NEW_PERSON])
        again = directory.call("POST", f"/{self.REALM}/users", self.new_person(self.NEW_PERSON))
        self.expect("directory: the same prefix again is refused (HTTP)", again.status, 409)
        for bad in ["a..b", ".ana", "ana-", "x" * 65, "ana@example.com", "ana b"]:
            reply = directory.call("POST", f"/{self.REALM}/users", self.new_person(bad))
            self.expect(f"directory: refuses username {bad[:20]!r} (HTTP)", reply.status, 400)
        self.check_must_change(directory)
        self.check_disable_and_delete(directory)

    def check_must_change(self, directory: AccountSettings) -> None:
        user_id = directory.user_id(self.NEW_PERSON)
        claims = self.verifier.claims(self.password_grant(self.NEW_PERSON, self.NEW_PASSWORD)["access_token"])
        self.expect("new person: must-change claim is true", claims.get(self.MUST_CHANGE_CLAIM), True)
        account = self.admin_client(self.ACCOUNT_CLIENT, self.account_secret)
        self.expect("account: changes the password (HTTP)", account.reset_password(user_id, self.TEMPORARY_PASSWORD), 204)
        self.expect("account: clears the must-change attribute (HTTP)", self.set_must_change(account, user_id, None), 204)
        claims = self.verifier.claims(self.password_grant(self.NEW_PERSON, self.TEMPORARY_PASSWORD)["access_token"])
        self.expect("new person: no must-change claim after the change", self.MUST_CHANGE_CLAIM in claims, False)
        self.expect("directory: resets the password (HTTP)", directory.reset_password(user_id, self.NEW_PASSWORD), 204)
        self.expect("directory: sets must-change again (HTTP)", self.set_must_change(directory, user_id, "true"), 204)
        claims = self.verifier.claims(self.password_grant(self.NEW_PERSON, self.NEW_PASSWORD)["access_token"])
        self.expect("new person: must-change claim is true again", claims.get(self.MUST_CHANGE_CLAIM), True)
        self.expect("directory: may not set must-change to another value (HTTP)", self.set_must_change(directory, user_id, "maybe"), 400)

    def check_disable_and_delete(self, directory: AccountSettings) -> None:
        user_id = directory.user_id(self.NEW_PERSON)
        refresh_token = self.password_grant(self.NEW_PERSON, self.NEW_PASSWORD)["refresh_token"]
        self.expect("directory: logs the person out (HTTP)", directory.logout(user_id), 204)
        self.expect("refresh after the directory logout (HTTP)", self.refresh_reply(refresh_token).status, 400)
        self.expect("directory: disables the person (HTTP)", self.set_enabled(directory, user_id, False), 204)
        time.sleep(self.QUICK_FAILURE_GAP_SECONDS)
        self.expect("a disabled person cannot sign in (HTTP)", self.password_reply(self.NEW_PERSON, self.NEW_PASSWORD).status, 400)
        self.expect("directory: deletes the person (HTTP)", directory.call("DELETE", f"/{self.REALM}/users/{user_id}").status, 204)
        self.expect("directory: the person is gone", self.usernames(directory, self.NEW_PERSON), [])

    def check_account_settings(self) -> None:
        account = self.admin_client(self.ACCOUNT_CLIENT, self.account_secret)
        self.expect("account: finds bo by prefix", bool(account.user_id("bo")), True)
        self.expect("account: may not list clients (HTTP)", account.get(f"/{self.REALM}/clients"), 403)
        self.expect("account: may not read the master realm (HTTP)", account.get("/master/users"), 403)
        user_id = account.user_id("carol")
        self.password_grant("carol", self.PEOPLE["carol"])
        session_ids = account.session_ids(user_id)
        self.expect("account: lists carol's sessions", len(session_ids) > 0, True)
        self.expect("account: ends one session (HTTP)", account.end_session(session_ids[0]), 204)
        self.expect("account: resets carol's password (HTTP)", account.reset_password(user_id, self.TEMPORARY_PASSWORD), 204)
        self.expect("carol signs in with the new password (HTTP)", self.password_reply("carol", self.TEMPORARY_PASSWORD).status, 200)
        self.expect("account: sets carol's password back (HTTP)", account.reset_password(user_id, self.PEOPLE["carol"]), 204)
        refresh_token = self.password_grant("carol", self.PEOPLE["carol"])["refresh_token"]
        self.expect("account: logs carol out everywhere (HTTP)", account.logout(user_id), 204)
        self.expect("refresh after the logout (HTTP)", self.refresh_reply(refresh_token).status, 400)

    def check_otp(self) -> None:
        username, password = self.OTP_PERSON, self.PEOPLE[self.OTP_PERSON]
        account = self.admin_client(self.ACCOUNT_CLIENT, self.account_secret)
        user_id = account.user_id(username)
        secret = Totp.new_secret()
        Totp.wait_for_fresh_step(self.FRESH_STEP_MARGIN_SECONDS)
        code = Totp.code(secret)
        self.expect("OTP endpoint refuses a wrong code", account.store_otp(user_id, secret, Totp.wrong(code)), (400, "invalid_code"))
        self.expect("OTP endpoint refuses a short secret", account.store_otp(user_id, secret[:20], code), (400, "invalid_request"))
        self.expect("OTP endpoint refuses other digits", account.store_otp(user_id, secret, code, digits=8), (400, "invalid_request"))
        self.expect("OTP endpoint answers 404 for an unknown user", account.store_otp("no-such-user", secret, code), (404, "user_not_found"))
        self.expect("OTP endpoint stores the TOTP", account.store_otp(user_id, secret, code), (204, ""))
        self.expect("OTP endpoint refuses a second TOTP", account.store_otp(user_id, secret, code), (409, "otp_exists"))
        self.expect("the TOTP is listed", "otp" in account.credential_types(user_id), True)
        self.check_otp_audit(user_id, secret)
        person = AccountSettings(self.http, self.base_url, self.REALM, "", client=self.IDENTITY_CLIENT)
        person.token = self.password_grant("ana", self.PEOPLE["ana"])["access_token"]
        self.expect("OTP endpoint refuses a person's own token (401 or 403)",
                    account_status(person.store_otp(user_id, Totp.new_secret(), code)) in (401, 403), True)
        self.expect("admin API refuses a person's own token (401 or 403)", person.get(f"/{self.REALM}/users") in (401, 403), True)
        self.check_otp_sign_in(username, password, code, secret)
        directory = self.admin_client(self.DIRECTORY_CLIENT, self.directory_secret)
        self.expect("directory: removes the TOTP credential (HTTP)", directory.delete_credentials(user_id, "otp"), 204)
        self.expect("OTP endpoint refuses a used code again", account.store_otp(user_id, secret, code), (400, "code_used"))
        self.expect("sign-in without a code after the removal (HTTP)", self.password_reply(username, password).status, 200)

    def check_otp_sign_in(self, username: str, password: str, confirmation: str, secret: str) -> None:
        attempts = [
            ("sign-in without the code", None, 400),
            ("the confirmation code cannot sign in", confirmation, 400),
            ("sign-in with the code one step back", Totp.code(secret, -1), 200),
            ("sign-in with the code one step ahead", Totp.code(secret, 1), 200),
            ("the same code a second time", Totp.code(secret, 1), 400),
            ("a code two steps ahead", Totp.code(secret, 2), 400),
        ]
        for name, code, expected in attempts:
            time.sleep(self.QUICK_FAILURE_GAP_SECONDS)
            self.expect(f"{name} (HTTP)", self.password_reply(username, password, code).status, expected)
        time.sleep(self.QUICK_FAILURE_GAP_SECONDS)

    def check_otp_audit(self, user_id: str, secret: str) -> None:
        client_id = os.environ.get("KC_BOOTSTRAP_ADMIN_CLIENT_ID", "")
        client_secret = os.environ.get("KC_BOOTSTRAP_ADMIN_CLIENT_SECRET", "")
        if not client_id or not client_secret:
            print("no admin client in the environment; audit event not checked")
            return
        admin = AccountSettings(self.http, self.base_url, "master", client_secret, client=client_id)
        admin.sign_in()
        events = admin.call("GET", f"/{self.REALM}/admin-events?operationTypes=CREATE&resourceTypes=USER&max=50").json_list()
        matching = [event for event in events if f"users/{user_id}/credentials/" in event.get("resourcePath", "")]
        self.expect("admin event for the stored TOTP", len(matching) > 0, True)
        self.expect("the admin event holds no secret", any(secret in json.dumps(event) for event in matching), False)

    def check_lockout(self) -> None:
        username, password = self.LOCK_PERSON, self.PEOPLE[self.LOCK_PERSON]
        for _ in range(self.FAILURES_BEFORE_LOCK):
            self.password_reply(username, "wrong-password-0")
        locked = self.password_reply(username, password)
        self.expect("locked account with the right password (HTTP, error, description)",
                    (locked.status, locked.json().get("error"), locked.json().get("error_description")), (400, "invalid_grant", "Invalid user credentials"))
        account = self.admin_client(self.ACCOUNT_CLIENT, self.account_secret)
        user_id = account.user_id(username)
        self.expect("lock status says disabled", account.lock_status(user_id).get("disabled"), True)
        self.expect("other people are not locked: ana signs in (HTTP)", self.password_reply("ana", self.PEOPLE["ana"]).status, 200)
        self.expect("clear the lock (HTTP)", account.clear_lock(user_id), 204)
        self.expect("sign-in after the lock is cleared (HTTP)", self.password_reply(username, password).status, 200)

    def admin_client(self, client: str, secret: str) -> AccountSettings:
        settings = AccountSettings(self.http, self.base_url, self.REALM, secret, client=client)
        self.expect(f"{client}: client-credentials token (HTTP)", settings.sign_in(), 200)
        return settings

    def new_person(self, username: str) -> dict:
        return {
            "username": username, "firstName": "Dan", "lastName": "New", "enabled": True,
            "attributes": {self.MUST_CHANGE_ATTRIBUTE: ["true"]},
            "credentials": [{"type": "password", "value": self.NEW_PASSWORD, "temporary": False}],
        }

    def remove_person_if_present(self, directory: AccountSettings, username: str) -> None:
        user_id = directory.user_id(username)
        if user_id:
            directory.call("DELETE", f"/{self.REALM}/users/{user_id}")

    def usernames(self, client: AccountSettings, search: str) -> list:
        reply = client.call("GET", f"/{self.REALM}/users?" + urllib.parse.urlencode({"username": search, "exact": "true"}))
        return [user["username"] for user in reply.json_list()]

    def set_must_change(self, client: AccountSettings, user_id: str, value: str | None) -> int:
        user = client.call("GET", f"/{self.REALM}/users/{user_id}").json()
        attributes = dict(user.get("attributes") or {})
        attributes.pop(self.MUST_CHANGE_ATTRIBUTE, None)
        if value is not None:
            attributes[self.MUST_CHANGE_ATTRIBUTE] = [value]
        user["attributes"] = attributes
        return client.call("PUT", f"/{self.REALM}/users/{user_id}", user).status

    def set_enabled(self, client: AccountSettings, user_id: str, enabled: bool) -> int:
        user = client.call("GET", f"/{self.REALM}/users/{user_id}").json()
        user["enabled"] = enabled
        return client.call("PUT", f"/{self.REALM}/users/{user_id}", user).status

    def discovery_status(self, realm: str) -> int:
        return self.http.send(f"{self.base_url}/realms/{realm}/.well-known/openid-configuration").status

    def browser_outcome(self, reply: Reply) -> tuple[str, bool]:
        if 300 <= reply.status < 400:
            location = reply.headers.get("Location", "")
            return f"redirect to {location}", "error=unauthorized_client" in location
        if reply.status == 200 or 400 <= reply.status < 500:
            offered = self.LOGIN_FORM in reply.text()
            return f"HTTP {reply.status}, login form {'present' if offered else 'absent'}", not offered
        return f"HTTP {reply.status}", False

    def password_grant(self, username: str, password: str) -> dict:
        return self.require_ok(self.password_reply(username, password), f"password grant for {username}")

    def password_reply(self, username: str, password: str, totp: str | None = None) -> Reply:
        fields = {
            "grant_type": "password", "client_id": self.IDENTITY_CLIENT, "client_secret": self.identity_secret,
            "username": username, "password": password, "scope": "openid",
        }
        if totp is not None:
            fields["totp"] = totp
        return self.http.post_form(self.urls.token, fields)

    def refresh_reply(self, refresh_token: str) -> Reply:
        return self.http.post_form(self.urls.token, {
            "grant_type": "refresh_token", "client_id": self.IDENTITY_CLIENT, "client_secret": self.identity_secret,
            "refresh_token": refresh_token,
        })

    def client_credentials(self, client: str, secret: str) -> Reply:
        return self.http.post_form(self.urls.token, {"grant_type": "client_credentials"}, (client, secret))

    def introspect(self, token: str) -> object:
        reply = self.http.post_form(self.urls.introspection, {"token": token}, (self.IDENTITY_CLIENT, self.identity_secret))
        return reply.json().get("active", f"HTTP {reply.status}")

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
        print(f"=== oidc check (realm mesail): {'all passed' if not self.failures else str(len(self.failures)) + ' failed'}")
        for failure in self.failures:
            print(f"FAILED {failure}")
        return 1 if self.failures else 0

    @staticmethod
    def require_ok(reply: Reply, action: str) -> dict:
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


if __name__ == "__main__":
    sys.exit(MesailRealmCheck(
        os.environ["MESAIL_KEYCLOAK_URL"], os.environ["MESAIL_IDENTITY_CLIENT_SECRET"],
        os.environ["MESAIL_ACCOUNT_CLIENT_SECRET"], os.environ["MESAIL_KEYCLOAK_DIRECTORY_SECRET"],
        os.environ.get("CHECK_LEGACY_REALMS") == "true",
    ).run())
