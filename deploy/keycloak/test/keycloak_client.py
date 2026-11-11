import base64
import hashlib
import hmac
import json
import os
import struct
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from email.message import Message
from typing import NamedTuple


class Totp:
    STEP_SECONDS = 30

    @staticmethod
    def code(secret: str, steps_from_now: int = 0) -> str:
        key = base64.b32decode(secret + "=" * (-len(secret) % 8))
        counter = int(time.time()) // Totp.STEP_SECONDS + steps_from_now
        digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
        offset = digest[-1] & 15
        return str((struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF) % 1000000).zfill(6)

    @staticmethod
    def new_secret() -> str:
        return base64.b32encode(os.urandom(20)).decode().rstrip("=")

    @staticmethod
    def wrong(code: str) -> str:
        return str((int(code) + 500000) % 1000000).zfill(6)

    @staticmethod
    def wait_for_fresh_step(margin_seconds: int) -> None:
        left = Totp.STEP_SECONDS - int(time.time()) % Totp.STEP_SECONDS
        if left < margin_seconds:
            time.sleep(left + 1)


def account_status(result: tuple[int, str]) -> int:
    return result[0]


class AccountSettings:
    CLIENT = "mesail-account"

    def __init__(self, http: "Http", base_url: str, realm: str, secret: str, client: str = CLIENT) -> None:
        self.client = client
        self.http = http
        self.base_url = base_url
        self.realm = realm
        self.secret = secret
        self.token = ""

    def sign_in(self) -> int:
        reply = self.http.post_form(f"{self.base_url}/realms/{self.realm}/protocol/openid-connect/token",
                                    {"grant_type": "client_credentials"}, (self.client, self.secret))
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

    def store_otp(self, user_id: str, secret: str, code: str, digits: int = 6) -> tuple[int, str]:
        body = {"secret": secret, "digits": digits, "period": 30, "algorithm": "HmacSHA1", "code": code}
        reply = self.call("PUT", f"/{self.realm}/mesail/users/{user_id}/otp", body)
        return reply.status, reply.json().get("error", "")

    def credential_types(self, user_id: str) -> list:
        return [credential["type"] for credential in self.call("GET", f"/{self.realm}/users/{user_id}/credentials").json_list()]

    def session_ids(self, user_id: str) -> list:
        return [session["id"] for session in self.call("GET", f"/{self.realm}/users/{user_id}/sessions").json_list()]

    def end_session(self, session_id: str) -> int:
        return self.call("DELETE", f"/{self.realm}/sessions/{session_id}").status

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
