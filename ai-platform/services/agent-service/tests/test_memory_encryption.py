"""Tests for app/memory/encryption.py: memory is encrypted at rest, bound to its place, and keys can rotate."""

import base64
import os

import pytest
from langchain_core.messages import AIMessage, HumanMessage

from app.memory.config import load_memory_settings
from app.memory.encryption import AesGcmCipher, KeyRing, build_key_ring, create_serializer, field_aad
from app.memory.errors import MemoryDataError

KEY = bytes(range(32))
OTHER_KEY = bytes(range(1, 33))
AAD = field_aad("3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34", "turn-1", "question")


# Key ring: PostgreSQL columns

def test_round_trip_with_the_key_id_first():

    ring = KeyRing(1, {1: KEY})

    sealed = ring.seal_text("Synthetic question?", AAD)

    assert sealed[0] == 1
    assert ring.key_id_of(sealed) == 1
    assert ring.unseal_text(sealed, AAD) == "Synthetic question?"


def test_sealed_value_holds_no_plaintext_and_differs_each_time():

    ring = KeyRing(1, {1: KEY})

    first = ring.seal_text("Synthetic question?", AAD)
    second = ring.seal_text("Synthetic question?", AAD)

    assert b"Synthetic" not in first
    assert first != second


def test_value_moved_to_another_conversation_or_field_fails():

    ring = KeyRing(1, {1: KEY})
    sealed = ring.seal_text("Synthetic question?", AAD)

    with pytest.raises(MemoryDataError):
        ring.unseal_text(sealed, field_aad("01936f3e-7b2a-7c4d-9e5f-0a1b2c3d4e5f", "turn-1", "question"))

    with pytest.raises(MemoryDataError):
        ring.unseal_text(sealed, field_aad("3f2b8c1e-9a4d-4f6b-8c2e-1d7a5b9e0f34", "turn-1", "answer"))


def test_changed_data_fails():

    ring = KeyRing(1, {1: KEY})
    sealed = ring.seal_text("Synthetic question?", AAD)
    changed = sealed[:-1] + bytes([sealed[-1] ^ 1])

    with pytest.raises(MemoryDataError):
        ring.unseal_text(changed, AAD)


def test_unknown_key_and_short_values_fail():

    sealed = KeyRing(1, {1: KEY}).seal_text("Synthetic question?", AAD)

    with pytest.raises(MemoryDataError):
        KeyRing(2, {2: OTHER_KEY}).unseal_text(sealed, AAD)

    with pytest.raises(MemoryDataError):
        KeyRing(1, {1: KEY}).unseal(b"\x01short", AAD)


def test_rotated_ring_reads_old_values_and_seals_with_the_new_key():

    old = KeyRing(1, {1: KEY}).seal_text("Synthetic question?", AAD)
    rotated = KeyRing(2, {2: OTHER_KEY, 1: KEY})

    resealed = rotated.seal(rotated.unseal(old, AAD), AAD)

    assert rotated.unseal_text(old, AAD) == "Synthetic question?"
    assert rotated.key_id_of(resealed) == 2


def test_current_key_must_be_in_the_ring():

    with pytest.raises(ValueError):
        KeyRing(2, {1: KEY})


def test_ring_is_built_from_the_settings():

    settings = load_memory_settings(environ={
        "AGENT_MEMORY_DATABASE_URL": "postgres://app:pw@127.0.0.1/govai_agent",
        "AGENT_MEMORY_DATA_KEY": base64.b64encode(OTHER_KEY).decode(),
        "AGENT_MEMORY_DATA_KEY_ID": "2",
        "AGENT_MEMORY_OLD_DATA_KEYS": "1:" + base64.b64encode(KEY).decode(),
    })
    old = KeyRing(1, {1: KEY}).seal_text("Synthetic question?", AAD)

    ring = build_key_ring(settings)

    assert ring.current_id == 2
    assert ring.unseal_text(old, AAD) == "Synthetic question?"


# Cipher: checkpoint values

def test_cipher_names_its_key():

    cipher = AesGcmCipher(KeyRing(3, {3: KEY}))

    name, ciphertext = cipher.encrypt(b"synthetic memory")

    assert name == "aesgcm-k3"
    assert b"synthetic" not in ciphertext
    assert cipher.decrypt(name, ciphertext) == b"synthetic memory"


@pytest.mark.parametrize("name", ["aesgcm", "aes-k1", "aesgcm-kx", "aesgcm-k9"])
def test_unknown_cipher_or_key_fails(name):

    with pytest.raises(MemoryDataError):
        AesGcmCipher(KeyRing(1, {1: KEY})).decrypt(name, os.urandom(40))


def test_wrong_key_fails():

    name, ciphertext = AesGcmCipher(KeyRing(1, {1: KEY})).encrypt(b"synthetic memory")

    with pytest.raises(MemoryDataError):
        AesGcmCipher(KeyRing(1, {1: OTHER_KEY})).decrypt(name, ciphertext)


# Serializer

def test_messages_survive_the_serializer_encrypted():

    serializer = create_serializer(KeyRing(1, {1: KEY}))
    messages = [HumanMessage(content="Synthetic question?"), AIMessage(content="Synthetic answer.")]

    stored_type, stored = serializer.dumps_typed(messages)
    restored = serializer.loads_typed((stored_type, stored))

    assert stored_type.endswith("+aesgcm-k1")
    assert b"Synthetic" not in stored
    assert restored == messages


def test_serializer_with_a_rotated_ring_reads_old_values():

    stored = create_serializer(KeyRing(1, {1: KEY})).dumps_typed({"text": "Synthetic question?"})

    restored = create_serializer(KeyRing(2, {2: OTHER_KEY, 1: KEY})).loads_typed(stored)

    assert restored == {"text": "Synthetic question?"}
