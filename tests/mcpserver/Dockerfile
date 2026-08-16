# Throwaway container for chatz's e2e harness: runs the same FastMCP test
# server (server.py) that the Go integration tests exec via python3 directly,
# but reachable over HTTP from inside docker compose so a REAL (not mocked)
# MCP server can be wired into the admin UI e2e flow. streamable-HTTP only —
# stdio has no meaning as a standalone container.
FROM python:3.12-slim

RUN pip install --no-cache-dir --root-user-action=ignore "mcp[cli]==1.27.1"

WORKDIR /srv
COPY server.py .

RUN useradd -r -u 10001 mcpserver
USER mcpserver

EXPOSE 8000

ENTRYPOINT ["python3", "server.py", "http", "8000", "0.0.0.0"]
