import type { components } from "./schema";

type ErrorEnvelope = components["schemas"]["Error"];

// ApiError wraps the backend's {code, message, details} envelope plus the HTTP
// status, so callers can switch on the machine-readable code.
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly details?: Record<string, unknown>;

  constructor(status: number, envelope: ErrorEnvelope) {
    super(envelope.message);
    this.name = "ApiError";
    this.code = envelope.code;
    this.status = status;
    this.details = envelope.details;
  }
}

// isErrorEnvelope narrows an unknown response body to the error envelope shape.
export function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const candidate = value as Record<string, unknown>;

  return (
    typeof candidate.code === "string" && typeof candidate.message === "string"
  );
}

// toApiError builds an ApiError from an HTTP status and an unknown error body,
// falling back to a generic envelope when the body is not the expected shape.
export function toApiError(status: number, body: unknown): ApiError {
  if (isErrorEnvelope(body)) {
    return new ApiError(status, body);
  }

  return new ApiError(status, {
    code: "UNKNOWN",
    message: `request failed with status ${status}`,
  });
}
