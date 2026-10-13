
import uvicorn
from fastapi import FastAPI
from app.config import (
    AI_SERVICE_HOST,
    AI_SERVICE_PORT,
    AI_SERVICE_ENV,
)

# Initialize FastAPI application
app = FastAPI(
    title="GovAI Agent Service",
    description="AI Agent service for the GovAI Agentic Platform",
    version="0.1.0",
)


# Root endpoint
@app.get("/")
async def root():
    return {
        "success": True,
        "message": "GovAI AI Agent Service is running"
    }


# Health check endpoint
@app.get("/health")
async def health_check():
    return {
        "success": True,
        "service": "govai-ai-agent",
        "status": "healthy",
        "environment": AI_SERVICE_ENV
    }


# Run server directly
if __name__ == "__main__":
    uvicorn.run(
        "app.main:app",
        host=AI_SERVICE_HOST,
        port=AI_SERVICE_PORT,
        reload=AI_SERVICE_ENV == "development",
    )
