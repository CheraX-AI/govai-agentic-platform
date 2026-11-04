# GovAI agent service: the Knowledge Agent graph, with the Chainlit chat UI.
#
# The default command starts the Chainlit chat; the agent-studio compose service
# runs the same image with `langgraph dev` for LangGraph Studio instead.
# Settings (NVIDIA, knowledge service, LangSmith) come from the environment at
# runtime, never from the image. See ai-platform/docker-compose.ai.yml.

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

# Copy the service code and the LangGraph entry point
COPY agent.py langgraph.json ./
COPY app ./app
COPY scripts ./scripts

# Run as a non-root user that owns /app, where Chainlit and LangGraph write their local state
RUN useradd --create-home --uid 10001 govai \
    && chown -R govai:govai /app
USER govai

# Chainlit chat (8502); LangGraph dev server (2024) when run by agent-studio
EXPOSE 8502 2024
CMD ["chainlit", "run", "scripts/chat_ui.py", "--host", "0.0.0.0", "--port", "8502", "--headless"]
