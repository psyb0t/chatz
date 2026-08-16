// MCP server connection-status constants — mirror the api.yml MCPServer.status
// and statusReason enums so the store + admin page reference the same strings.

export const MCP_STATUS_CONNECTING = "connecting";
export const MCP_STATUS_CONNECTED = "connected";
export const MCP_STATUS_FAILED = "failed";
export const MCP_STATUS_DISABLED = "disabled";

export const MCP_REASON_UNREACHABLE = "unreachable";
export const MCP_REASON_ACCESS_DENIED = "access_denied";
export const MCP_REASON_NOT_RESPONDING = "not_responding";
export const MCP_REASON_FAILED = "failed";

export interface MCPHealthTelemetry {
  lastConnectionAttemptAt?: string;
  lastSuccessfulConnectionAt?: string;
  lastConnectionFailureAt?: string;
  lastConnectionLatencyMs?: number;
  lastError?: string;
}

export const MCP_HEALTH_LAST_ATTEMPT = "Last attempt";
export const MCP_HEALTH_LAST_SUCCESS = "Last success";
export const MCP_HEALTH_LAST_FAILURE = "Last failure";
export const MCP_HEALTH_CONNECT_LATENCY = "Connect latency";
export const MCP_HEALTH_LAST_ERROR = "Last error";

// mcpHealthDetails converts optional API telemetry into stable, concise admin
// labels. Invalid timestamps are skipped rather than rendering browser-specific
// "Invalid Date" text.
export function mcpHealthDetails(server: MCPHealthTelemetry): string[] {
  const details: string[] = [];
  const attemptedAt = formatMCPHealthTime(server.lastConnectionAttemptAt);
  const succeededAt = formatMCPHealthTime(server.lastSuccessfulConnectionAt);
  const failedAt = formatMCPHealthTime(server.lastConnectionFailureAt);

  if (attemptedAt !== undefined) {
    details.push(`${MCP_HEALTH_LAST_ATTEMPT}: ${attemptedAt}`);
  }

  if (succeededAt !== undefined) {
    details.push(`${MCP_HEALTH_LAST_SUCCESS}: ${succeededAt}`);
  }

  if (failedAt !== undefined) {
    details.push(`${MCP_HEALTH_LAST_FAILURE}: ${failedAt}`);
  }

  if (server.lastConnectionLatencyMs !== undefined) {
    details.push(
      `${MCP_HEALTH_CONNECT_LATENCY}: ${server.lastConnectionLatencyMs} ms`,
    );
  }

  if (server.lastError !== undefined && server.lastError.length > 0) {
    details.push(`${MCP_HEALTH_LAST_ERROR}: ${server.lastError}`);
  }

  return details;
}

function formatMCPHealthTime(value: string | undefined): string | undefined {
  if (value === undefined || Number.isNaN(Date.parse(value))) {
    return undefined;
  }

  return new Date(value).toISOString();
}

// mcpStatusLabel maps a server status to its display label.
export function mcpStatusLabel(status: string): string {
  switch (status) {
    case MCP_STATUS_CONNECTED:
      return "Connected";
    case MCP_STATUS_CONNECTING:
      return "Connecting…";
    case MCP_STATUS_FAILED:
      return "Failed";
    case MCP_STATUS_DISABLED:
      return "Disabled";
    default:
      return status;
  }
}

// mcpReasonLabel maps a failure reason to a human phrase for the row tooltip.
export function mcpReasonLabel(reason: string): string {
  switch (reason) {
    case MCP_REASON_UNREACHABLE:
      return "unreachable";
    case MCP_REASON_ACCESS_DENIED:
      return "access denied";
    case MCP_REASON_NOT_RESPONDING:
      return "not responding";
    default:
      return "failed";
  }
}
