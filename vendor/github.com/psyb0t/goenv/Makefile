MIN_TEST_COVERAGE := 90

.PHONY: all dep lint lint-fix test test-coverage docs help

all: dep lint test-coverage ## Run dep, lint and test

dep: ## Get project dependencies
	@echo "Getting project dependencies..."
	@go mod tidy

lint: ## Lint all Golang files
	@echo "Linting all Go files..."
	@out=$$(go fix -diff ./... 2>&1); \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		echo "go fix found issues. Run 'make lint-fix' to apply."; \
		exit 1; \
	fi
	@go vet ./...

lint-fix: ## Lint all Golang files and fix
	@echo "Linting and fixing all Go files..."
	@go fix ./...
	@go vet ./...

test: ## Run all tests
	@echo "Running all tests..."
	@go test -race ./...

test-coverage: ## Run tests with coverage check. Fails if coverage is below the threshold.
	@echo "Running tests with coverage check..."
	@trap 'rm -f coverage.txt' EXIT; \
	go test -race -coverprofile=coverage.txt ./...; \
	if [ $$? -ne 0 ]; then \
		echo "Test failed. Exiting."; \
		exit 1; \
	fi; \
	result=$$(go tool cover -func=coverage.txt | grep -oP 'total:\s+\(statements\)\s+\K\d+' || echo "0"); \
	pct=$$(go tool cover -func=coverage.txt | grep -oP 'total:\s+\(statements\)\s+\K[0-9.]+' || echo "0"); \
	echo "$$pct" > coverage-percent.txt; \
	if [ $$result -eq 0 ]; then \
		echo "No test coverage information available."; \
		exit 0; \
	elif [ $$result -lt $(MIN_TEST_COVERAGE) ]; then \
		echo "FAIL: Coverage $$result% is less than the minimum $(MIN_TEST_COVERAGE)%"; \
		exit 1; \
	fi

docs: ## Build the docs site into .testing/docs-site with GitHub Pages' Jekyll image
	@echo "Building the docs site..."
	@mkdir -p .testing
	@docker run --rm \
		--user "$$(id -u):$$(id -g)" \
		-e HOME=/tmp \
		-e GITHUB_WORKSPACE=/work \
		-e GITHUB_REPOSITORY=psyb0t/goenv \
		-e INPUT_SOURCE=docs \
		-e INPUT_DESTINATION=.testing/docs-site \
		-e INPUT_VERBOSE=false \
		-v "$(CURDIR):/work" \
		ghcr.io/actions/jekyll-build-pages:v1.0.13@sha256:6791ebfd912185ed59bfb5fb102664fa872496b79f87ff8b9cfba292a7345041

help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
