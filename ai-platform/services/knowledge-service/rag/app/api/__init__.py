"""Internal HTTP API of the knowledge service.

Modules:

- main.py  FastAPI app: POST /api/v1/retrieve and GET /health

Only agent-service calls this API; citizens and the frontend talk to the agent.
"""
