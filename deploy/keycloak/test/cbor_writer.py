class CborWriter:
    UNSIGNED = 0
    NEGATIVE = 1
    BYTES = 2
    TEXT = 3
    MAP = 5
    ONE_BYTE_LENGTH = 24
    TWO_BYTE_LENGTH = 25

    @staticmethod
    def encode(value: object) -> bytes:
        if isinstance(value, bool) or value is None:
            raise TypeError("CborWriter writes only integers, bytes, text and maps")
        if isinstance(value, int):
            return CborWriter.integer(value)
        if isinstance(value, bytes):
            return CborWriter.head(CborWriter.BYTES, len(value)) + value
        if isinstance(value, str):
            encoded = value.encode()
            return CborWriter.head(CborWriter.TEXT, len(encoded)) + encoded
        return CborWriter.mapping(value)

    @staticmethod
    def integer(value: int) -> bytes:
        if value >= 0:
            return CborWriter.head(CborWriter.UNSIGNED, value)
        return CborWriter.head(CborWriter.NEGATIVE, -1 - value)

    @staticmethod
    def mapping(value: dict) -> bytes:
        body = b"".join(CborWriter.encode(key) + CborWriter.encode(item) for key, item in value.items())
        return CborWriter.head(CborWriter.MAP, len(value)) + body

    @staticmethod
    def head(major: int, length: int) -> bytes:
        if length < CborWriter.ONE_BYTE_LENGTH:
            return bytes([major << 5 | length])
        if length < 256:
            return bytes([major << 5 | CborWriter.ONE_BYTE_LENGTH, length])
        return bytes([major << 5 | CborWriter.TWO_BYTE_LENGTH]) + length.to_bytes(2, "big")
