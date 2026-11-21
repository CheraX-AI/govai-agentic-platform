"""Tests for app/memory/encryption.py: memory is encrypted at rest and unreadable without the key."""

import os

import pytest
from langchain_core.messages import AIMessage, HumanMessage
from langgraph.checkpoint.memory import InMemorySaver

from app.memory.encryption import AesGcmCipher, MemoryDataError, create_serializer

KEY = bytes(range(32))
OTHER_KEY = bytes(range(1, 33))


# Cipher

def test_round_trip():

    cipher = AesGcmCipher(KEY)

    name, ciphertext = cipher.encrypt(b"synthetic memory")

    assert name == "aesgcm"
    assert cipher.decrypt(name, ciphertext) == b"synthetic memory"


def test_ciphertext_holds_no_plaintext_and_differs_each_time():

    cipher = AesGcmCipher(KEY)

    first = cipher.encrypt(b"synthetic memory")[1]
    second = cipher.encrypt(b"synthetic memory")[1]

    assert b"synthetic" not in first
    assert first != second


def test_wrong_key_fails():

    name, ciphertext = AesGcmCipher(KEY).encrypt(b"synthetic memory")

    with pytest.raises(MemoryDataError):
        AesGcmCipher(OTHER_KEY).decrypt(name, ciphertext)


def test_changed_data_fails():

    name, ciphertext = AesGcmCipher(KEY).encrypt(b"synthetic memory")
    changed = ciphertext[:-1] + bytes([ciphertext[-1] ^ 1])

    with pytest.raises(MemoryDataError):
        AesGcmCipher(KEY).decrypt(name, changed)


def test_unknown_cipher_fails():

    with pytest.raises(MemoryDataError):
        AesGcmCipher(KEY).decrypt("aes", os.urandom(40))


# Serializer

def test_messages_survive_the_serializer():

    serializer = create_serializer(KEY)
    messages = [HumanMessage(content="Synthetic question?"), AIMessage(content="Synthetic answer.")]

    stored_type, stored = serializer.dumps_typed(messages)
    restored = serializer.loads_typed((stored_type, stored))

    assert stored_type.endswith("+aesgcm")
    assert b"Synthetic" not in stored
    assert restored == messages


def test_checkpoint_saved_with_another_key_cannot_be_loaded():

    # Save a checkpoint with one key, then read the same rows with another
    saver = InMemorySaver(serde=create_serializer(KEY))
    config = {"configurable": {"thread_id": "t1", "checkpoint_ns": ""}}
    checkpoint = {
        "v": 4,
        "id": "1f000000-0000-6000-8000-000000000001",
        "ts": "2026-11-22T00:00:00+00:00",
        "channel_values": {"messages": [HumanMessage(content="Synthetic question?")]},
        "channel_versions": {"messages": 1},
        "versions_seen": {},
        "updated_channels": None,
    }
    saver.put(config, checkpoint, {}, {"messages": 1})

    saver.serde = create_serializer(OTHER_KEY)

    with pytest.raises(MemoryDataError):
        saver.get_tuple(config)
