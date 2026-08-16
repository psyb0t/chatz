#!/usr/bin/env python3
"""Minimal MCP server for chatz's integration tests.

Exposes tools over either transport, selected by argv:
  - echo(text)     -> "echo: <text>"
  - marker()       -> the CHATZ_MCP_MARKER env var (proves stdio env injection)
  - sleep(seconds) -> blocks for seconds then returns "slept: <seconds>"
                       (proves the manager's idle-heartbeat fires during a
                       slow, still-in-flight tool call)

Usage:
  server.py stdio
  server.py http <port> [host]   # streamable-HTTP (SSE over HTTP) at /mcp
                                  # host defaults to 127.0.0.1 (test-fixture
                                  # default); pass 0.0.0.0 to run standalone
                                  # (e.g. containerized, reachable off-host).
"""

import os
import sys
import time

from mcp.server.fastmcp import FastMCP


def build(host: str, port: int) -> FastMCP:
    mcp = FastMCP("chatz-test", host=host, port=port)

    @mcp.tool()
    def echo(text: str) -> str:
        """Echo the input text back, prefixed."""
        return f"echo: {text}"

    @mcp.tool()
    def marker() -> str:
        """Return CHATZ_MCP_MARKER from the environment (may be empty)."""
        return os.environ.get("CHATZ_MCP_MARKER", "")

    @mcp.tool()
    def sleep(seconds: float) -> str:
        """Block for seconds, then return a marker string."""
        time.sleep(seconds)

        return f"slept: {seconds}"

    return mcp


def main() -> None:
    mode = sys.argv[1] if len(sys.argv) > 1 else "stdio"

    if mode == "http":
        port = int(sys.argv[2])
        host = sys.argv[3] if len(sys.argv) > 3 else "127.0.0.1"
        build(host, port).run(transport="streamable-http")
        return

    build("127.0.0.1", 0).run(transport="stdio")


if __name__ == "__main__":
    main()
