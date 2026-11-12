import os
import time
import urllib.parse

from keycloak_client import AccountSettings, Reply
from software_authenticator import SoftwareAuthenticator


class PasskeyCheck:
    PERSON = "pk.person"
    PASSWORD = "passkey-person-password-1"
    LABEL = "Mac"
    NEW_LABEL = "MacBook"
    MOST_PASSKEYS = 10
    LABEL_MAX_LENGTH = 64
    QUICK_FAILURE_GAP_SECONDS = 1.5
    OTHER_ORIGIN = "https://other.example"
    ANDROID_ORIGIN = "android:apk-key-hash:" + "Wx2Lh6sJ-vq3xq3l2Kc3w9Rk1sJcU1pJ8m7Qk0yZb0E"
    WRONG_GRANT = (400, "invalid_grant", "Invalid user credentials")
    FAILURES_OF_THE_PERSON = 5

    def __init__(self, check: "MesailRealmCheck") -> None:
        self.check = check
        self.relying_party = os.environ.get("MESAIL_PASSKEY_RP_ID", "localhost")
        self.web_origin = f"https://{self.relying_party}"
        self.account: AccountSettings | None = None
        self.user_id = ""

    def run(self) -> None:
        directory = self.check.admin_client(self.check.DIRECTORY_CLIENT, self.check.directory_secret)
        self.account = self.check.admin_client(self.check.ACCOUNT_CLIENT, self.check.account_secret)
        self.user_id = self.new_person(directory)
        try:
            mac = self.check_registration()
            self.check_sign_in(mac)
            self.check_refused_sign_ins(mac)
            self.check_rename(mac)
            phone = self.check_android()
            self.check_limit(mac)
            self.check_removal(mac, phone)
            self.check_directory_removes_all(directory)
        finally:
            directory.call("DELETE", f"/{self.check.REALM}/users/{self.user_id}")

    def check_registration(self) -> dict:
        mac = SoftwareAuthenticator(self.relying_party, self.web_origin)
        self.expect_error("register: an unknown person", self.register(mac, person="no-such-user"), 404, "user_not_found")
        self.expect_error("register: an origin that is not allowed", self.register(mac, client_origin=self.OTHER_ORIGIN), 400, "invalid_passkey")
        self.expect_error("register: a malformed allowed origin", self.register(mac, origins=["ftp://localhost"]), 400, "invalid_request")
        self.expect_error("register: the challenge does not match", self.register(mac, signed_challenge=SoftwareAuthenticator.new_challenge()), 400, "invalid_passkey")
        self.expect_error("register: without user verification", self.register(mac, flags=SoftwareAuthenticator.USER_PRESENT), 400, "invalid_passkey")
        self.expect_error("register: an empty label", self.register(mac, label=" "), 400, "invalid_request")
        self.expect_error("register: a label over 64 characters", self.register(mac, label="x" * (self.LABEL_MAX_LENGTH + 1)), 400, "invalid_request")
        challenge = SoftwareAuthenticator.new_challenge()
        created = self.register(mac, challenge=challenge)
        passkey = created.json()
        self.check.expect("register: the passkey is stored (HTTP)", created.status, 201)
        self.check.expect("register: the answer names the WebAuthn credential id", passkey.get("credentialId"), SoftwareAuthenticator.b64(mac.credential_id))
        self.check.expect("register: label, created, never used", (passkey.get("label"), isinstance(passkey.get("createdAt"), int), passkey.get("lastUsedAt")), (self.LABEL, True, None))
        self.expect_error("register: the same challenge twice", self.register(SoftwareAuthenticator(self.relying_party, self.web_origin), challenge=challenge), 400, "challenge_used")
        self.expect_error("register: the same credential twice", self.register(mac), 409, "passkey_exists")
        self.check.expect("list: one passkey", [item.get("passkeyId") for item in self.passkeys()], [passkey.get("passkeyId")])
        self.check_person_token_refused()
        return {"authenticator": mac, "passkeyId": passkey.get("passkeyId")}

    def check_person_token_refused(self) -> None:
        person = AccountSettings(self.check.http, self.check.base_url, self.check.REALM, "", client=self.check.IDENTITY_CLIENT)
        person.token = self.check.password_grant(self.PERSON, self.PASSWORD)["access_token"]
        path = f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys"
        self.check.expect("passkey endpoints refuse a person's own token (401 or 403)", person.call("GET", path).status in (401, 403), True)

    def check_sign_in(self, mac: dict) -> None:
        challenge = SoftwareAuthenticator.new_challenge()
        assertion = mac["authenticator"].assert_as(challenge, self.user_id)
        token = self.check.require_ok(self.sign_in(assertion, challenge), "passkey sign-in")
        claims = self.check.check_signature("passkey", token["access_token"])
        self.check.expect("passkey sign-in: sub is the person", claims.get("sub"), self.user_id)
        self.check.expect("passkey sign-in: azp is mesail-identity", claims.get("azp"), self.check.IDENTITY_CLIENT)
        self.check.expect("passkey sign-in: preferred_username", claims.get("preferred_username"), self.PERSON)
        self.check.expect("passkey sign-in: a refresh token", bool(token.get("refresh_token")), True)
        self.check.expect("list: lastUsedAt is set after the sign-in", isinstance(self.passkeys()[0].get("lastUsedAt"), int), True)
        self.expect_refused("a replayed assertion", self.sign_in(assertion, challenge))
        mac["lastChallenge"] = challenge

    def check_refused_sign_ins(self, mac: dict) -> None:
        authenticator = mac["authenticator"]
        other = SoftwareAuthenticator(self.relying_party, self.web_origin)
        ana = self.account.user_id("ana")
        attempts = [
            ("a wrong signature", lambda challenge: (authenticator.assert_as(challenge, self.user_id, signer=other), challenge, None)),
            ("an origin that is not allowed", lambda challenge: (authenticator.assert_as(challenge, self.user_id, origin=self.OTHER_ORIGIN), challenge, None)),
            ("a challenge that does not match", lambda challenge: (authenticator.assert_as(SoftwareAuthenticator.new_challenge(), self.user_id), challenge, None)),
            ("without user verification", lambda challenge: (authenticator.assert_as(challenge, self.user_id, flags=SoftwareAuthenticator.USER_PRESENT), challenge, None)),
            ("an unknown user handle", lambda challenge: (authenticator.assert_as(challenge, "no-such-user"), challenge, None)),
            ("the passkey with another person's user handle", lambda challenge: (authenticator.assert_as(challenge, ana), challenge, None)),
            ("an unknown passkey", lambda challenge: (other.assert_as(challenge, self.user_id), challenge, None)),
            ("no allowed origin", lambda challenge: (authenticator.assert_as(challenge, self.user_id), challenge, [])),
        ]
        for name, make in attempts:
            assertion, challenge, origins = make(SoftwareAuthenticator.new_challenge())
            self.expect_refused(name, self.sign_in(assertion, challenge, origins))
        lock = self.account.lock_status(self.user_id)
        self.check.expect("brute-force detection counted the failed passkey sign-ins", lock.get("numFailures", 0) >= self.FAILURES_OF_THE_PERSON, True)
        self.check_lockout(authenticator, other)
        self.check.expect("clear the lock (HTTP)", self.account.clear_lock(self.user_id), 204)
        self.check.expect("clear ana's lock (HTTP)", self.account.clear_lock(ana), 204)
        self.check.expect("the password still signs the person in (HTTP)", self.check.password_reply(self.PERSON, self.PASSWORD).status, 200)

    def check_lockout(self, authenticator: SoftwareAuthenticator, other: SoftwareAuthenticator) -> None:
        for _ in range(self.check.FAILURES_BEFORE_LOCK):
            challenge = SoftwareAuthenticator.new_challenge()
            self.sign_in(authenticator.assert_as(challenge, self.user_id, signer=other), challenge)
            time.sleep(self.QUICK_FAILURE_GAP_SECONDS)
        self.check.expect("passkey failures lock the person (lock status disabled)", self.account.lock_status(self.user_id).get("disabled"), True)
        challenge = SoftwareAuthenticator.new_challenge()
        self.expect_refused("a valid passkey while the person is locked", self.sign_in(authenticator.assert_as(challenge, self.user_id), challenge))

    def check_rename(self, mac: dict) -> None:
        path = f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys/{mac['passkeyId']}"
        renamed = self.account.call("PUT", path, {"label": self.NEW_LABEL})
        self.check.expect("rename (HTTP, label)", (renamed.status, renamed.json().get("label")), (200, self.NEW_LABEL))
        self.expect_error("rename: an empty label", self.account.call("PUT", path, {"label": ""}), 400, "invalid_request")
        missing = f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys/no-such-passkey"
        self.expect_error("rename: an unknown passkey", self.account.call("PUT", missing, {"label": self.LABEL}), 404, "not_found")

    def check_android(self) -> dict:
        phone = SoftwareAuthenticator(self.relying_party, self.ANDROID_ORIGIN)
        both = [self.web_origin, self.ANDROID_ORIGIN]
        created = self.register(phone, origins=both, label="Pixel")
        self.check.expect("register: an Android passkey (HTTP)", created.status, 201)
        challenge = SoftwareAuthenticator.new_challenge()
        self.check.expect("sign-in with the Android origin allowed (HTTP)", self.sign_in(phone.assert_as(challenge, self.user_id), challenge, both).status, 200)
        challenge = SoftwareAuthenticator.new_challenge()
        self.expect_refused("the Android origin when only the web origin is allowed", self.sign_in(phone.assert_as(challenge, self.user_id), challenge))
        return {"authenticator": phone, "passkeyId": created.json().get("passkeyId")}

    def check_limit(self, mac: dict) -> None:
        self.expect_error("register: a label the person already uses", self.register(SoftwareAuthenticator(self.relying_party, self.web_origin), label="Pixel"), 409, "label_exists")
        for number in range(len(self.passkeys()), self.MOST_PASSKEYS):
            self.register(SoftwareAuthenticator(self.relying_party, self.web_origin), label=f"Device {number}")
        self.check.expect("list: 10 passkeys", len(self.passkeys()), self.MOST_PASSKEYS)
        self.expect_error("register: an 11th passkey", self.register(SoftwareAuthenticator(self.relying_party, self.web_origin), label="Device 11"), 409, "too_many")

    def check_removal(self, mac: dict, phone: dict) -> None:
        path = f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys/{mac['passkeyId']}"
        ana_path = f"/{self.check.REALM}/mesail/users/{self.account.user_id('ana')}/passkeys/{phone['passkeyId']}"
        self.expect_error("remove: another person's passkey", self.account.call("DELETE", ana_path), 404, "not_found")
        self.check.expect("remove a passkey (HTTP)", self.account.call("DELETE", path).status, 204)
        self.expect_error("remove: the same passkey again", self.account.call("DELETE", path), 404, "not_found")
        self.check.expect("list: 9 passkeys after the removal", len(self.passkeys()), self.MOST_PASSKEYS - 1)
        challenge = SoftwareAuthenticator.new_challenge()
        self.expect_refused("a removed passkey", self.sign_in(mac["authenticator"].assert_as(challenge, self.user_id), challenge))
        self.check.expect("clear the lock (HTTP)", self.account.clear_lock(self.user_id), 204)

    def check_directory_removes_all(self, directory: AccountSettings) -> None:
        path = f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys"
        self.check.expect("directory: removes all passkeys (HTTP)", directory.call("DELETE", path).status, 204)
        self.check.expect("list: no passkey left", self.passkeys(), [])
        user = directory.call("GET", f"/{self.check.REALM}/users/{self.user_id}").json()
        self.check.expect("no last-used value left", "mesail.passkeyLastUsed" in (user.get("attributes") or {}), False)
        self.check.expect("credentials: no webauthn-passwordless left", "webauthn-passwordless" in self.account.credential_types(self.user_id), False)

    def new_person(self, directory: AccountSettings) -> str:
        self.check.remove_person_if_present(directory, self.PERSON)
        body = {"username": self.PERSON, "enabled": True, "credentials": [{"type": "password", "value": self.PASSWORD, "temporary": False}]}
        self.check.expect("directory: creates the passkey test person (HTTP)", directory.call("POST", f"/{self.check.REALM}/users", body).status, 201)
        return directory.user_id(self.PERSON)

    def register(self, authenticator: SoftwareAuthenticator, challenge: str | None = None, signed_challenge: str | None = None,
                 client_origin: str | None = None, origins: list | None = None, label: str = LABEL,
                 flags: int = SoftwareAuthenticator.USER_PRESENT | SoftwareAuthenticator.USER_VERIFIED, person: str | None = None) -> Reply:
        challenge = challenge or SoftwareAuthenticator.new_challenge()
        body = {
            "credentialJson": authenticator.create(signed_challenge or challenge, client_origin, flags),
            "challenge": challenge, "origins": origins if origins is not None else [self.web_origin], "label": label,
        }
        return self.account.call("POST", f"/{self.check.REALM}/mesail/users/{person or self.user_id}/passkeys", body)

    def passkeys(self) -> list:
        return self.account.call("GET", f"/{self.check.REALM}/mesail/users/{self.user_id}/passkeys").json_list()

    def sign_in(self, assertion: str, challenge: str, origins: list | None = None) -> Reply:
        fields = {
            "grant_type": "password", "client_id": self.check.IDENTITY_CLIENT, "client_secret": self.check.identity_secret,
            "scope": "openid", "passkey_assertion": assertion, "passkey_challenge": challenge,
            "passkey_origins": " ".join(origins if origins is not None else [self.web_origin]),
        }
        headers = {"Content-Type": "application/x-www-form-urlencoded"}
        return self.check.http.send(self.check.urls.token, urllib.parse.urlencode(fields).encode(), headers)

    def expect_refused(self, name: str, reply: Reply) -> None:
        body = reply.json()
        self.check.expect(f"passkey sign-in refused like a wrong password: {name}", (reply.status, body.get("error"), body.get("error_description")), self.WRONG_GRANT)
        time.sleep(self.QUICK_FAILURE_GAP_SECONDS)

    def expect_error(self, name: str, reply: Reply, status: int, error: str) -> None:
        self.check.expect(f"{name} (HTTP, error)", (reply.status, reply.json().get("error")), (status, error))
