"""Fixed error responses and request IDs shared by the API routes; bodies never echo the input."""

import re
import uuid

from fastapi.responses import JSONResponse

REQUEST_ID_HEADER = "X-Request-ID"
REQUEST_ID_PATTERN = re.compile(r"^[A-Za-z0-9-]{1,64}$")

UNAVAILABLE_DETAIL = "The GovAI assistant is unavailable."
NOT_FOUND_DETAIL = "Conversation not found."
BUSY_DETAIL = "This question is still being answered. Try again shortly."

# Seconds core-service tells the citizen to wait before asking the same question again
BUSY_RETRY_AFTER = "2"


# Public functions

def read_request_id(request):
    """core-service's request ID when it is safe, otherwise a new one."""

    request_id = request.headers.get(REQUEST_ID_HEADER, "")
    if not REQUEST_ID_PATTERN.match(request_id):
        request_id = uuid.uuid4().hex

    return request_id


def json_response(content, request_id, status_code=200, headers=None):

    response = JSONResponse(
        status_code=status_code,
        content=content,
        headers={REQUEST_ID_HEADER: request_id, **(headers or {})},
    )

    return response


def unavailable(request_id):
    """503: core-service shows the citizen "Chat temporarily unavailable"."""

    response = json_response({"detail": UNAVAILABLE_DETAIL}, request_id, status_code=503)

    return response


def not_found(request_id):
    """404 for a conversation that does not exist or belongs to another citizen."""

    response = json_response({"detail": NOT_FOUND_DETAIL}, request_id, status_code=404)

    return response


def busy(request_id):
    """409: the same question is still being answered; core-service asks the citizen to wait."""

    response = json_response({"detail": BUSY_DETAIL}, request_id, status_code=409, headers={"Retry-After": BUSY_RETRY_AFTER})

    return response
