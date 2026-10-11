// Error with an HTTP status code, picked up by the error-handling middleware.
export default class ApiError extends Error {
  constructor(statusCode, message) {
    super(message);
    this.statusCode = statusCode;
  }
}
