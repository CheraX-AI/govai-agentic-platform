const express = require("express");
const cors = require("cors");

const app = express();

const PORT = process.env.PORT || 5000;
const NODE_ENV = process.env.NODE_ENV || "development";

const allowedOrigins = process.env.CORS_ORIGIN
  ? process.env.CORS_ORIGIN.split(",")
  : ["http://localhost:3000", "http://localhost:5173"];

app.use(express.json());

app.use(
  cors({
    origin: allowedOrigins,
    credentials: true,
  })
);

app.get("/api/health", (req, res) => {
  res.status(200).json({
    success: true,
    message: "GovAI backend is running",
    environment: NODE_ENV,
    timestamp: new Date().toISOString(),
  });
});

app.use((req, res) => {
  res.status(404).json({
    success: false,
    message: "Route not found",
  });
});

app.listen(PORT, () => {
  console.log(`GovAI backend server running on port ${PORT}`);
});