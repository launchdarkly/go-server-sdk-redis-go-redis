
GOLANGCI_LINT_VERSION=v1.60.1

LINTER=./bin/golangci-lint
LINTER_VERSION_FILE=./bin/.golangci-lint-version-$(GOLANGCI_LINT_VERSION)

ALL_SOURCES := $(shell find * -type f -name "*.go")

COVERAGE_PROFILE_RAW=./build/coverage_raw.out
COVERAGE_PROFILE_RAW_HTML=./build/coverage_raw.html
COVERAGE_PROFILE_FILTERED=./build/coverage.out
COVERAGE_PROFILE_FILTERED_HTML=./build/coverage.html
COVERAGE_ENFORCER_FLAGS=-skipcode "// COVERAGE" -packagestats -filestats -showcode

.PHONY: build clean test test-coverage lint

build:
	go build ./...

clean:
	go clean

test:
	CGO_ENABLED=1 go test -count=1 -race -v ./...

test-coverage: $(COVERAGE_PROFILE_RAW)
	if [ -x "$(GOPATH)/bin/go-coverage-enforcer)" ]; then go get github.com/launchdarkly-labs/go-coverage-enforcer; fi
	$(GOPATH)/bin/go-coverage-enforcer $(COVERAGE_ENFORCER_FLAGS) -outprofile $(COVERAGE_PROFILE_FILTERED) $(COVERAGE_PROFILE_RAW)
	go tool cover -html $(COVERAGE_PROFILE_FILTERED) -o $(COVERAGE_PROFILE_FILTERED_HTML)
	go tool cover -html $(COVERAGE_PROFILE_RAW) -o $(COVERAGE_PROFILE_RAW_HTML)

$(COVERAGE_PROFILE_RAW): $(ALL_SOURCES)
	@mkdir -p ./build
	go test -coverprofile $(COVERAGE_PROFILE_RAW) ./... >/dev/null

$(LINTER_VERSION_FILE):
	rm -f $(LINTER)
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | bash -s $(GOLANGCI_LINT_VERSION)
	touch $(LINTER_VERSION_FILE)

lint: $(LINTER_VERSION_FILE)
	$(LINTER) run ./...
