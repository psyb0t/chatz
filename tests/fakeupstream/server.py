#!/usr/bin/env python3
"""Minimal OpenAI-compatible streaming fixture for chatz's e2e harness.

Real (not mocked-in-process) HTTP server so the backend's actual upstream
discovery (internal/pkg/upstreams.Discover -> llmclient openai.ListModels,
which pages the official OpenAI SDK's Models.ListAutoPaging over GET
/v1/models) has something real and reachable to hit — letting the e2e browser
exercise the model picker (search/filter/select) against genuine model IDs
without needing a real OpenAI API key or a live LLM. Its completion endpoint
sends a first text delta, pauses long enough for a browser stop/reload test,
then finishes normally when the client remains connected.

Usage: server.py <port>
"""

import json
import os
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MODELS = ["e2e-fake-gpt", "e2e-fake-mini", "e2e-fake-vision"]
PARTIAL_RESPONSE = "Recovered partial answer."
COMPLETED_RESPONSE = " The stream completed."
STREAM_PAUSE_SECONDS = 5
FAIL_FIRST_STREAM_ENV = "CHATZ_API_FAIL_FIRST_STREAM"
# When set, the assistant turn is exactly this text and streams in one chunk
# with no pause, instead of the partial/pause/completed script. Lets a test
# drive the browser with a specific assistant payload — notably a malformed
# ```spec block, which no canned showcase response can express.
RESPONSE_TEXT_ENV = "CHATZ_API_RESPONSE_TEXT"
FAILURE_DELAY_SECONDS = 1.5
# The OpenAI SDK retries a transient 5xx twice before Elelem sees it; Elelem
# then makes three provider attempts. Exhaust all nine requests of the first
# application turn so the browser receives a real terminal stream error.
FAILURE_ATTEMPTS_PER_STREAM_TURN = 9


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    failure_attempts = 0
    failure_lock = threading.Lock()

    def do_GET(self) -> None:
        if self.path.rstrip("/") != "/v1/models":
            self.send_response(404)
            self.end_headers()
            return

        body = json.dumps(
            {
                "object": "list",
                "data": [
                    {"id": m, "object": "model", "owned_by": "e2e"}
                    for m in MODELS
                ],
            }
        ).encode()

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self) -> None:
        if self.path.rstrip("/") != "/v1/chat/completions":
            self.send_response(404)
            self.end_headers()
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
            request = json.loads(self.rfile.read(content_length))
        except (ValueError, json.JSONDecodeError):
            self.send_response(400)
            self.end_headers()
            return

        if not request.get("stream"):
            self._send_json_completion(request)
            return

        if self._should_fail_first_attempt(request):
            time.sleep(FAILURE_DELAY_SECONDS)
            self.log_message(
                "forced stream failure %d/%d",
                Handler.failure_attempts,
                FAILURE_ATTEMPTS_PER_STREAM_TURN,
            )
            self.send_response(503)
            self.send_header("Content-Length", "0")
            self.send_header("Connection", "close")
            self.end_headers()
            return

        self._stream_completion(request)

    @classmethod
    def _should_fail_first_attempt(cls, request: dict) -> bool:
        del request

        if os.getenv(FAIL_FIRST_STREAM_ENV) != "true":
            return False

        with cls.failure_lock:
            if cls.failure_attempts >= FAILURE_ATTEMPTS_PER_STREAM_TURN:
                return False

            cls.failure_attempts += 1
            return True

    def _send_json_completion(self, request: dict) -> None:
        body = json.dumps(
            {
                "id": "e2e-completion",
                "object": "chat.completion",
                "model": request.get("model", MODELS[0]),
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": PARTIAL_RESPONSE + COMPLETED_RESPONSE,
                        },
                        "finish_reason": "stop",
                    }
                ],
            }
        ).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _stream_completion(self, request: dict) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()

        scripted = os.getenv(RESPONSE_TEXT_ENV, "")
        if scripted:
            if not self._write_chunk(request, scripted, "stop"):
                return
        else:
            if not self._write_chunk(request, PARTIAL_RESPONSE, None):
                return

            time.sleep(STREAM_PAUSE_SECONDS)

            if not self._write_chunk(request, COMPLETED_RESPONSE, "stop"):
                return

        try:
            self.wfile.write(b"data: [DONE]\n\n")
            self.wfile.flush()
        except BrokenPipeError:
            self.log_message("client closed streamed completion")

    def _write_chunk(
        self,
        request: dict,
        content: str,
        finish_reason: str | None,
    ) -> bool:
        event = {
            "id": "e2e-completion",
            "object": "chat.completion.chunk",
            "model": request.get("model", MODELS[0]),
            "choices": [
                {
                    "index": 0,
                    "delta": {"content": content},
                    "finish_reason": finish_reason,
                }
            ],
        }

        try:
            self.wfile.write(f"data: {json.dumps(event)}\n\n".encode())
            self.wfile.flush()
        except BrokenPipeError:
            self.log_message("client closed streamed completion")
            return False

        return True

    def log_message(self, fmt: str, *args: object) -> None:
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))


def main() -> None:
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8000
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()


if __name__ == "__main__":
    main()
