# GovAI knowledge service (RAG): internal retrieval API for agent-service.
#
# Settings (NVIDIA, ChromaDB, SeaweedFS) come from the environment at runtime,
# never from the image. See ai-platform/docker-compose.ai.yml.

FROM python:3.13-slim

# No .pyc files, unbuffered logs, no pip cache in the image
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# Install the dependencies first, so this layer is reused when only the code changes
COPY requirements.txt .
RUN pip install -r requirements.txt

# Copy the service code
COPY app ./app
COPY scripts ./scripts

# Run as a non-root user
RUN useradd --create-home --uid 10001 govai \
    && chown -R govai:govai /app
USER govai

# Listen on every container interface; compose does not publish this port, so only
# containers on the GovAI network (the agent service) can reach the API
EXPOSE 8020
CMD ["uvicorn", "app.api.main:app", "--host", "0.0.0.0", "--port", "8020"]
