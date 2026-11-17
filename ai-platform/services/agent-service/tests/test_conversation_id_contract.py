"""Contract test: the agent accepts exactly the conversation IDs that backend/pkg/chatcontract accepts.

Both sides read the same cases from backend/pkg/chatcontract/testdata/conversation_ids.json.
"""

import json
from pathlib import Path

import pytest
from pydantic import ValidationError

from app.api.schemas import ChatRequest

# The repository root is four folders above this file (tests, agent-service, services, ai-platform)
SHARED_CASES = Path(__file__).resolve().parents[4] / "backend" / "pkg" / "chatcontract" / "testdata" / "conversation_ids.json"


# Helper functions

def load_cases():

    # Skip outside the full repository, e.g. when only agent-service is checked out
    if not SHARED_CASES.is_file():
        pytest.skip("backend/pkg/chatcontract/testdata/conversation_ids.json is not available")

    with open(SHARED_CASES, "r", encoding="utf-8") as file:
        cases = json.load(file)

    return cases


# Tests

def test_go_valid_ids_are_accepted():

    cases = load_cases()

    for conversation_id in cases["valid"]:
        request = ChatRequest(message="Synthetic question", conversation_id=conversation_id)
        assert request.conversation_id == conversation_id


def test_go_invalid_ids_are_rejected():

    cases = load_cases()

    for conversation_id in cases["invalid"]:
        with pytest.raises(ValidationError):
            ChatRequest(message="Synthetic question", conversation_id=conversation_id)
