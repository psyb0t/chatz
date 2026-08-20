# Throwaway api-only container: a real (stdlib-only, no deps) OpenAI-compatible
# /v1/models stub so the model picker can be exercised against genuine model
# IDs without a real API key or LLM upstream. See server.py for scope.
FROM python:3.12-slim

WORKDIR /srv
COPY server.py .

RUN useradd -r -u 10002 fakeupstream
USER fakeupstream

EXPOSE 8000

ENTRYPOINT ["python3", "server.py", "8000"]
