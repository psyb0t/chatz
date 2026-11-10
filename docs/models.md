---
title: "Models and upstreams"
description: "How chatz connects OpenAI-compatible and Anthropic upstreams: the CHATZ_UPSTREAMS JSON, model metadata, fallbacks, the default model and health."
nav_order: 4
---

# Models and upstreams

An upstream is one model endpoint: OpenAI, an OpenAI-compatible server such as Ollama, vLLM or a gateway like [AIGate](https://github.com/psyb0t/aigate), or Anthropic. chatz talks to them through the [elelem](https://github.com/psyb0t/elelem) drivers, asks each one for its model list at startup, and merges the lists into the model picker.

## The CHATZ_UPSTREAMS format

`CHATZ_UPSTREAMS` is a JSON array with one object per upstream:

```json
[
  {
    "name": "openai",
    "provider": "openai",
    "baseUrl": "https://api.openai.com/v1",
    "apiKeyEnv": "OPENAI_API_KEY"
  },
  {
    "name": "anthropic",
    "provider": "anthropic",
    "apiKeyEnv": "ANTHROPIC_API_KEY"
  },
  {
    "name": "aigate",
    "provider": "openai",
    "baseUrl": "http://aigate:4000",
    "apiKeyEnv": "AIGATE_TOKEN"
  },
  {
    "name": "ollama",
    "provider": "openai",
    "baseUrl": "http://localhost:11434/v1"
  }
]
```

In `.env` the whole array goes on one line after `CHATZ_UPSTREAMS=`.

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | A label for the upstream, unique in the array. It shows up in the model list and in health reports. |
| `provider` | yes | `openai` for OpenAI and every OpenAI-compatible API, or `anthropic`. |
| `baseUrl` | no | The API root. Leave it out to use the provider's own default endpoint. |
| `apiKeyEnv` | no | The name of the environment variable that holds the API key. Leave it out for keyless endpoints such as a local Ollama. |
| `models` | no | Extra metadata for models this upstream lists. See below. |

Keys are only ever read from the variable that `apiKeyEnv` names, and each upstream uses only its own key and base URL. chatz does not fall back to the provider SDK's usual variables. If `apiKeyEnv` names a variable that is unset or empty, startup stops.

The AIGate example assumes chatz and AIGate share a Docker network. Otherwise use an address chatz can reach.

## Discovery

At startup chatz calls each upstream's model list, bounded by `CHATZ_UPSTREAM_CONNECT_TIMEOUT` and tried twice. The lists are merged in the order the upstreams appear:

- An upstream that fails discovery is skipped and marked degraded. The others still load, and startup continues.
- When two upstreams list the same model ID, the first one in the array serves it and the later one is skipped with a warning.
- Discovery runs only at startup. A model added on the provider side appears after a restart.

The list is served at `GET /api/v1/models`. A configuration error in the JSON, such as a missing name or an unknown provider, stops startup; [Troubleshooting](troubleshooting.md#upstream-configuration) lists each message.

## Model metadata

The optional `models` array attaches display names, limits, capabilities and prices to models the provider lists. An entry never adds a model the provider did not return; an entry for a model that is not discovered is ignored.

```json
{
  "name": "openai",
  "provider": "openai",
  "baseUrl": "https://api.openai.com/v1",
  "apiKeyEnv": "OPENAI_API_KEY",
  "models": [
    {
      "id": "analysis-model",
      "alias": "Deep",
      "contextWindow": 128000,
      "maxOutputTokens": 8192,
      "supportsTools": true,
      "supportsReasoning": true,
      "supportsFiles": true,
      "expectedFirstTokenLatencyMs": 900,
      "inputPricePerMillionTokens": {
        "amountSmallestUnit": 15,
        "currency": "USD"
      },
      "outputPricePerMillionTokens": {
        "amountSmallestUnit": 60,
        "currency": "USD"
      },
      "fallbackModels": ["backup-model"]
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `id` | Required. The model ID exactly as the provider lists it. Unique within the upstream. |
| `alias` | A friendly name for the model picker. |
| `contextWindow`, `maxOutputTokens` | Token limits shown to users. Must not be negative. |
| `supportsTools`, `supportsReasoning`, `supportsVision`, `supportsFiles` | Capability flags. A flag you leave out means unknown, not false. |
| `expectedFirstTokenLatencyMs` | A latency hint shown with the model. Must not be negative. |
| `inputPricePerMillionTokens`, `outputPricePerMillionTokens` | `amountSmallestUnit` is an integer in the currency's smallest unit (cents for USD), never a float. `currency` is a three-letter uppercase code. |
| `fallbackModels` | Model IDs to try, in order, when this model fails before streaming starts. |

Prices are display data. chatz records token counts per call but does not compute cost.

## Fallback models

When a request fails before any output has streamed, and the failure is a first-token timeout, a turn timeout, a rate limit or an unavailable model, chatz tries the next ID in `fallbackModels`. The stream reports a `retrying` status while it switches. A failure after output has started is not retried on another model.

Fallback IDs must be discovered models. An empty entry, a model that lists itself, or a duplicate entry stops startup, and so does a fallback that no upstream lists. Fallbacks of a model that is not discovered are ignored.

## Default model

`CHATZ_DEFAULT_MODEL` marks one discovered model as the default for new chats. The web app selects it, or the first model in the list when no default is set. If the default is not discovered, for example because its upstream was down at startup, chatz refuses to start.

## Timeouts and concurrency

| Setting | Applies to |
| --- | --- |
| `CHATZ_UPSTREAM_CONNECT_TIMEOUT` (`10s`) | Model discovery and the connection setup of each request. |
| `CHATZ_UPSTREAM_FIRST_TOKEN_TIMEOUT` (`45s`) | The wait for the first streamed token. |
| `CHATZ_UPSTREAM_TURN_TIMEOUT` (`5m`) | The whole model request. |
| `CHATZ_UPSTREAM_CONCURRENCY` (`8`) | Requests in flight per upstream. |

Each upstream has its own queue of concurrency slots. The first-token and turn timers start once a request holds a slot. A timed-out request ends the stream with an error the user can retry; see [Chats](chats.md#failures-and-retries).

## Upstream health

Admins can see each upstream's health at `GET /api/v1/upstreams` and on the readiness page in the web app. Every entry carries:

| Field | Meaning |
| --- | --- |
| `upstream` | The upstream name. |
| `state` | `unknown` before any call, `healthy` after a success, `degraded` after a failure. |
| `lastOperation` | `model_discovery`, `queue`, `stream` or `complete`. |
| `lastLatencyMs` | Latency of the last call. |
| `lastSuccessAt`, `lastFailureAt` | Timestamps of the last success and failure. |
| `lastFailureClass` | `cancelled`, `deadline_exceeded`, `first_token_timeout`, `turn_timeout`, `discovery_timeout` or `upstream_error`. |
| `consecutiveFailures` | Failures since the last success. |

The snapshot holds no keys, URLs or request contents.
