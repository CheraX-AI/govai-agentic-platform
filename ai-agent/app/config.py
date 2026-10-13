
import os
from dotenv import load_dotenv

load_dotenv()

AI_SERVICE_HOST = os.getenv("AI_SERVICE_HOST", "127.0.0.1")
AI_SERVICE_PORT = int(os.getenv("AI_SERVICE_PORT", "8002"))
AI_SERVICE_ENV = os.getenv("AI_SERVICE_ENV", "development")

BACKEND_URL = os.getenv("BACKEND_URL", "http://localhost:3000")
CHROMA_URL = os.getenv("CHROMA_URL", "http://localhost:8001")
