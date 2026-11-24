"""AES-256-GCM encryption of conversation memory, with a key ring so keys can be rotated."""

import os

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from langgraph.checkpoint.serde.encrypted import EncryptedSerializer

from app.memory.errors import MemoryDataError

NONCE_SIZE = 12
TAG_SIZE = 16
KEY_ID_SIZE = 1

# Checkpoint values name their key, e.g. "aesgcm-k1"; the "+" separator is reserved by EncryptedSerializer
CIPHER_PREFIX = "aesgcm-k"


# Key ring

class KeyRing:
    """The current key encrypts; every key in the ring can decrypt what it once encrypted."""

    def __init__(self, current_id, keys):

        if current_id not in keys:
            raise ValueError("The current key must be in the key ring")

        self.current_id = current_id
        self.ciphers = {}
        for key_id, key in keys.items():
            self.ciphers[key_id] = AESGCM(key)

    def seal(self, plaintext, aad):
        """Return key ID || nonce || ciphertext || tag; aad binds the value to where it is stored."""

        # A fresh random nonce for every value
        nonce = os.urandom(NONCE_SIZE)
        ciphertext = self.ciphers[self.current_id].encrypt(nonce, plaintext, aad)

        sealed = bytes([self.current_id]) + nonce + ciphertext

        return sealed

    def unseal(self, sealed, aad):
        """Decrypt a value from seal. Raises MemoryDataError for an unknown key or changed data."""

        if len(sealed) < KEY_ID_SIZE + NONCE_SIZE + TAG_SIZE:
            raise MemoryDataError("Memory value is too short")

        key_id = sealed[0]
        nonce = sealed[KEY_ID_SIZE:KEY_ID_SIZE + NONCE_SIZE]
        plaintext = self.open_with(key_id, nonce, sealed[KEY_ID_SIZE + NONCE_SIZE:], aad)

        return plaintext

    def seal_text(self, text, aad):

        sealed = self.seal(text.encode("utf-8"), aad)

        return sealed

    def unseal_text(self, sealed, aad):

        text = self.unseal(sealed, aad).decode("utf-8")

        return text

    def key_id_of(self, sealed):
        """The key a sealed value was written with, so old values can be found and re-sealed."""

        key_id = sealed[0]

        return key_id

    def open_with(self, key_id, nonce, ciphertext, aad):

        cipher = self.ciphers.get(key_id)
        if cipher is None:
            raise MemoryDataError("Memory was stored with a key that is not in the key ring")

        try:
            # The tag check fails for a wrong key or changed data
            plaintext = cipher.decrypt(nonce, ciphertext, aad)

        except InvalidTag:
            raise MemoryDataError("Memory cannot be decrypted") from None

        return plaintext


# Cipher

class AesGcmCipher:
    """The cipher LangGraph's EncryptedSerializer uses for checkpoint values: nonce || ciphertext || tag."""

    def __init__(self, ring):

        self.ring = ring

    def encrypt(self, plaintext):

        # The cipher name records the key, so a rotated key can still read older values
        nonce = os.urandom(NONCE_SIZE)
        ciphertext = self.ring.ciphers[self.ring.current_id].encrypt(nonce, plaintext, None)

        return f"{CIPHER_PREFIX}{self.ring.current_id}", nonce + ciphertext

    def decrypt(self, ciphername, ciphertext):

        key_id = cipher_key_id(ciphername)
        plaintext = self.ring.open_with(key_id, ciphertext[:NONCE_SIZE], ciphertext[NONCE_SIZE:], None)

        return plaintext


# Public functions

def build_key_ring(settings):
    """The key ring from MemorySettings: the current key plus the old keys kept for reading."""

    keys = {settings.data_key_id: settings.data_key.get_secret_value()}
    for key_id, key in settings.old_data_keys.items():
        keys[key_id] = key.get_secret_value()

    ring = KeyRing(settings.data_key_id, keys)

    return ring


def create_serializer(ring):
    """Checkpoint serializer that encrypts every stored value with the current key."""

    serializer = EncryptedSerializer(AesGcmCipher(ring))

    return serializer


def field_aad(*parts):
    """Associated data that ties a value to its conversation, row and field."""

    aad = "|".join(parts).encode("utf-8")

    return aad


# Helper functions

def cipher_key_id(ciphername):

    # "aesgcm-k<id>" names the key; anything else was not written by this service
    if not ciphername.startswith(CIPHER_PREFIX):
        raise MemoryDataError("Memory was stored with an unknown cipher")

    try:
        key_id = int(ciphername[len(CIPHER_PREFIX):])
    except ValueError:
        raise MemoryDataError("Memory was stored with an unknown cipher") from None

    return key_id
