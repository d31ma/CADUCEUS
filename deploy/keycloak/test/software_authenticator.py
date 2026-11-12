import base64
import hashlib
import json
import os
import struct
import subprocess
import tempfile

from cbor_writer import CborWriter


class SoftwareAuthenticator:
    USER_PRESENT = 0x01
    USER_VERIFIED = 0x04
    ATTESTED_DATA = 0x40
    ES256 = -7
    EC2_KEY = 2
    P256 = 1
    RAW_POINT_SIZE = 64
    CREDENTIAL_ID_SIZE = 32
    AAGUID = bytes(16)

    def __init__(self, relying_party: str, origin: str) -> None:
        self.relying_party = relying_party
        self.origin = origin
        self.credential_id = os.urandom(self.CREDENTIAL_ID_SIZE)
        self.sign_count = 0
        self.folder = tempfile.mkdtemp(prefix="mesail-passkey-")
        self.key_path = os.path.join(self.folder, "key.pem")
        subprocess.run(["openssl", "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", self.key_path], check=True, capture_output=True)

    def create(self, challenge: str, origin: str | None = None, flags: int = USER_PRESENT | USER_VERIFIED) -> str:
        client_data = self.client_data("webauthn.create", challenge, origin)
        attested = self.AAGUID + struct.pack(">H", len(self.credential_id)) + self.credential_id + self.cose_public_key()
        auth_data = self.auth_data(flags | self.ATTESTED_DATA) + attested
        attestation = CborWriter.encode({"fmt": "none", "attStmt": {}, "authData": auth_data})
        return json.dumps({
            "id": self.b64(self.credential_id), "rawId": self.b64(self.credential_id), "type": "public-key",
            "authenticatorAttachment": "platform", "clientExtensionResults": {},
            "response": {"clientDataJSON": self.b64(client_data), "attestationObject": self.b64(attestation), "transports": ["internal"]},
        })

    def assert_as(self, challenge: str, user_handle: str, origin: str | None = None,
                  flags: int = USER_PRESENT | USER_VERIFIED, signer: "SoftwareAuthenticator | None" = None) -> str:
        self.sign_count += 1
        client_data = self.client_data("webauthn.get", challenge, origin)
        auth_data = self.auth_data(flags)
        signature = (signer or self).sign(auth_data + hashlib.sha256(client_data).digest())
        return json.dumps({
            "id": self.b64(self.credential_id), "rawId": self.b64(self.credential_id), "type": "public-key",
            "authenticatorAttachment": "platform", "clientExtensionResults": {},
            "response": {"clientDataJSON": self.b64(client_data), "authenticatorData": self.b64(auth_data),
                         "signature": self.b64(signature), "userHandle": self.b64(user_handle.encode())},
        })

    def client_data(self, kind: str, challenge: str, origin: str | None) -> bytes:
        return json.dumps({"type": kind, "challenge": challenge, "origin": origin or self.origin, "crossOrigin": False}).encode()

    def auth_data(self, flags: int) -> bytes:
        return hashlib.sha256(self.relying_party.encode()).digest() + bytes([flags]) + struct.pack(">I", self.sign_count)

    def cose_public_key(self) -> bytes:
        der = subprocess.run(["openssl", "ec", "-in", self.key_path, "-pubout", "-outform", "DER"], check=True, capture_output=True).stdout
        point = der[-self.RAW_POINT_SIZE:]
        return CborWriter.encode({1: self.EC2_KEY, 3: self.ES256, -1: self.P256, -2: point[:32], -3: point[32:]})

    def sign(self, data: bytes) -> bytes:
        return subprocess.run(["openssl", "dgst", "-sha256", "-sign", self.key_path], input=data, check=True, capture_output=True).stdout

    @staticmethod
    def new_challenge() -> str:
        return SoftwareAuthenticator.b64(os.urandom(32))

    @staticmethod
    def b64(data: bytes) -> str:
        return base64.urlsafe_b64encode(data).decode().rstrip("=")
