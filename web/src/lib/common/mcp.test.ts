import { describe, expect, it } from "vitest";
import {
  MCP_HEALTH_CONNECT_LATENCY,
  MCP_HEALTH_LAST_ATTEMPT,
  MCP_HEALTH_LAST_ERROR,
  MCP_HEALTH_LAST_FAILURE,
  MCP_HEALTH_LAST_SUCCESS,
  mcpHealthDetails,
} from "./mcp";

describe("mcpHealthDetails", () => {
  it("renders each valid health field in a stable UTC form", () => {
    expect(
      mcpHealthDetails({
        lastConnectionAttemptAt: "2026-08-12T02:00:00Z",
        lastSuccessfulConnectionAt: "2026-08-12T02:00:01Z",
        lastConnectionFailureAt: "2026-08-12T02:01:00Z",
        lastConnectionLatencyMs: 120,
        lastError: "connection refused",
      }),
    ).toEqual([
      `${MCP_HEALTH_LAST_ATTEMPT}: 2026-08-12T02:00:00.000Z`,
      `${MCP_HEALTH_LAST_SUCCESS}: 2026-08-12T02:00:01.000Z`,
      `${MCP_HEALTH_LAST_FAILURE}: 2026-08-12T02:01:00.000Z`,
      `${MCP_HEALTH_CONNECT_LATENCY}: 120 ms`,
      `${MCP_HEALTH_LAST_ERROR}: connection refused`,
    ]);
  });

  it("omits invalid and missing telemetry", () => {
    expect(
      mcpHealthDetails({
        lastConnectionAttemptAt: "not-a-time",
        lastError: "",
      }),
    ).toEqual([]);
  });
});
