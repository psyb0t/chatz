# Go libraries and conventions

chatz depends on `psyb0t/aichteeteapee`, `psyb0t/common-go`, `psyb0t/ctxerrors`, `psyb0t/ctxscope` and `psyb0t/slogging`. Check them before declaring a constant, sentinel or helper.

- HTTP header names, content types, auth schemes and error code strings come from the `github.com/psyb0t/aichteeteapee` root package, for example `aichteeteapee.HeaderNameXRequestID`, `aichteeteapee.ErrorCodeBadRequest` and `aichteeteapee.ErrorCodeFromHTTPStatus(status)`. common-go's old `http` package is replaced by aichteeteapee.
- Generic sentinel errors (`ErrNotFound`, `ErrNotAuthenticated`, `ErrRateLimited`, `ErrAlreadyExists`, `ErrTimeout` and the rest) come from `github.com/psyb0t/common-go/errors`. Do not declare a local `ErrNotFound`; return `commonerrors.ErrNotFound`.
- Wrap errors with `ctxerrors.Wrap` or `ctxerrors.Wrapf`. `ctxerrors.MapError(from, to)` registers a translation that `Wrap` applies through `errors.Is`. `internal/pkg/http/server/error_map.go` uses it to collapse `gorm.ErrRecordNotFound` into `commonerrors.ErrNotFound`, so the HTTP layer has one not-found check.
- Log through the context logger: `ctxscope.GetLogger(ctx)`, with handlers from `github.com/psyb0t/slogging`. Process-wide fields (`service`, `version`, `commit`) are set once with `ctxscope.SetGlobal` in `internal/pkg/services/http-server`. Scope keys live in `internal/pkg/logging/scope_keys.go`.
- Logged attributes whose keys look like secrets are redacted by `internal/pkg/logging/redact.go`. Do not rely on that for chat text: debug logs can hold ordinary prompt content.
- Settings are fields on `config.Config` in `internal/pkg/config/config.go`, parsed by gonfiguration from `CHATZ_*` variables. A new setting also goes into `.env.example`, `docker-compose.yml` when the Compose stack needs it, and `docs/configuration.md`.
- The lint config is `.golangci.yml`, run through `go tool golangci-lint` inside `make lint`. Keep `//nolint` comments narrow and give the reason, as the existing ones do.
