"""AES-256-GCM encryption for conversation memory at rest."""

import os

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from langgraph.checkpoint.serde.encrypted import EncryptedSerializer

CIPHER_NAME = "aesgcm"
NONCE_SIZE = 12


# Errors

class MemoryDataError(Exception):
    """Stored memory cannot be decrypted, e.g. after a key change or tampering."""


# Cipher

class AesGcmCipher:
    """The cipher LangGraph's EncryptedSerializer uses: stores nonce || ciphertext || tag."""

    def __init__(self, key):

        self.aead = AESGCM(key)

    def encrypt(self, plaintext):

        # A fresh random nonce for every value
        nonce = os.urandom(NONCE_SIZE)
        ciphertext = self.aead.encrypt(nonce, plaintext, None)

        return CIPHER_NAME, nonce + ciphertext

    def decrypt(self, ciphername, ciphertext):

        if ciphername != CIPHER_NAME:
            raise MemoryDataError("Memory was stored with an unknown cipher")

        try:
            # The tag check fails for a wrong key or changed data
            plaintext = self.aead.decrypt(ciphertext[:NONCE_SIZE], ciphertext[NONCE_SIZE:], None)

        except InvalidTag:
            raise MemoryDataError("Memory cannot be decrypted") from None

        return plaintext


# Public functions

def create_serializer(key):
    """Checkpoint serializer that encrypts every stored value with the memory key."""

    serializer = EncryptedSerializer(AesGcmCipher(key))

    return serializer
