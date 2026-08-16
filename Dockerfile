# Production Dockerfile - Multi-stage build
#
# Stage 1 (web): build the SvelteKit static SPA with pnpm.
# Stage 2 (builder): compile the Go binary, embedding the SPA from stage 1 into
#   internal/pkg/webassets/dist so the single binary serves API + UI.
# Stage 3 (final): minimal non-root alpine runtime with just the static binary.

# --- web: build the SvelteKit static SPA ------------------------------------
# node MAJOR + pnpm version pinned per supply-chain discipline. --ignore-scripts
# blocks postinstall (the #1 JS supply-chain attack vector); --frozen-lockfile
# fails if pnpm-lock.yaml drifts.
FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS web
ARG PNPM_VERSION=10.15.0
RUN npm install -g pnpm@${PNPM_VERSION} && npm cache clean --force
WORKDIR /web
COPY web/ ./
RUN pnpm install --frozen-lockfile --ignore-scripts \
    && pnpm build

# --- builder: compile the static Go binary ----------------------------------
FROM golang:1.26.6-alpine@sha256:af8d6740070b8906d12eae1c3e3ea0957fb63f492051ea05e354c38ef9fe88df AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev

# Set working directory
WORKDIR /app

# Copy the module metadata and vendored dependencies. Production builds are
# offline and resolve exactly the checked-in dependency graph.
COPY go.mod go.sum ./
COPY vendor ./vendor

# Copy source code
COPY . .

# Build identity is deliberately non-secret metadata. CI supplies the release
# version and revision; local builds retain the explicit development defaults.
ARG APP_VERSION=dev
ARG GIT_COMMIT=

# Embed the freshly built SPA so go:embed picks up the real UI (replacing the
# committed placeholder). This dir is gitignored except for dist/index.html.
COPY --from=web /web/build ./internal/pkg/webassets/dist

# Build binary with static linking. Target ./cmd (the app's main) specifically —
# ./cmd/... would also match cmd/repogen (a second main pkg) and `-o <file>`
# rejects building multiple packages into one output file.
RUN CGO_ENABLED=0 go build -a -mod=vendor \
    -ldflags "-extldflags '-static' -X github.com/psyb0t/chatz/internal/pkg/buildinfo.Version=${APP_VERSION} -X github.com/psyb0t/chatz/internal/pkg/buildinfo.Commit=${GIT_COMMIT}" \
    -o ./build/app ./cmd

# Final stage - minimal runtime image
FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

# Install ca-certificates, create the non-root user, and pre-own the only
# writable runtime mount. Docker copies this ownership into a fresh named
# volume, so the read-only app process can create its SQLite file under /data.
RUN apk --no-cache add ca-certificates \
    && adduser -D -s /bin/sh appuser \
    && mkdir /data \
    && chown appuser:appuser /data

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/build/app .

# Change ownership to non-root user
RUN chown appuser:appuser /app/app

# Switch to non-root user
USER appuser

# Set entrypoint to the app binary
ENTRYPOINT ["./app"]

# Default command if no args provided
CMD ["--help"]
