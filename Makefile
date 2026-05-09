PROJECT := moma-enduro-workflows
MAKEDIR := hack/make
SHELL   := /bin/bash

.DEFAULT_GOAL := help
.PHONY: *

DBG_MAKEFILE ?=
ifeq ($(DBG_MAKEFILE),1)
    $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
    $(warning ***** $(shell date))
else
    # If we're not debugging the Makefile, don't echo recipes.
    MAKEFLAGS += -s
endif

define NEWLINE


endef

IGNORED_PACKAGES := \
	github.com/artefactual-sdps/moma-enduro-workflows/hack/%

PACKAGES := $(shell go list ./...)
TEST_PACKAGES := $(filter-out $(IGNORED_PACKAGES),$(PACKAGES))
TEST_IGNORED_PACKAGES := $(filter $(IGNORED_PACKAGES),$(PACKAGES))

# Configure bine.
export PATH := $(shell go tool bine path):$(PATH)

deps: # @HELP List available module dependency updates.
deps: tool-go-mod-outdated
	go list -u -m -json all | go-mod-outdated -direct -update

env: # @HELP Print Go env variables.
env:
	go env

fmt: # @HELP Format the project Go files with golangci-lint.
fmt: FMT_FLAGS ?=
fmt: tool-golangci-lint
	golangci-lint fmt $(FMT_FLAGS)

gosec: # @HELP Run gosec security scanner.
gosec: GOSEC_VERBOSITY ?= "-terse"
gosec: tool-gosec
	go tool bine upgrade gosec
	gosec $(GOSEC_VERBOSITY) -exclude-dir=hack ./...

govulncheck: # @HELP Run govulncheck security scanner.
govulncheck: tool-govulncheck
	go tool bine upgrade govulncheck
	govulncheck $(GOVULNCHECK_FLAGS) ./...

help: # @HELP Print this message.
	echo "TARGETS:"
	grep -hE '^.*:.*?# *@HELP' $(MAKEFILE_LIST) | sort | \
	    awk 'BEGIN {FS = ":.*?# *@HELP"}; { printf "  %-30s %s\n", $$1, $$2 };'

lint: # @HELP Lint the project Go files with golangci-lint.
lint: LINT_FLAGS ?= --timeout=5m --fix --output.text.colors
lint: tool-golangci-lint
	golangci-lint run $(LINT_FLAGS)

list-ignored-packages: # @HELP Print a list of packages ignored in testing.
list-ignored-packages:
	$(foreach PACKAGE,$(TEST_IGNORED_PACKAGES),@echo $(PACKAGE)$(NEWLINE))

list-tested-packages: # @HELP Print a list of packages being tested.
list-tested-packages:
	$(foreach PACKAGE,$(TEST_PACKAGES),@echo $(PACKAGE)$(NEWLINE))

pre-commit: # @HELP Check that code is ready to commit.
pre-commit:
	MOMA_ENDURO_INTEGRATION_TEST=1 $(MAKE) -j \
		fmt \
		gosec GOSEC_VERBOSITY="-quiet" \
		lint \
		shfmt \
		test-race

shfmt: SHELL_PROGRAMS := $(shell find $(CURDIR)/hack -name *.sh)
shfmt: tool-shfmt # @HELP Run shfmt to format shell programs in the hack directory.
	shfmt \
		--list \
		--write \
		--diff \
		--simplify \
		--language-dialect=posix \
		--indent=0 \
		--case-indent \
		--space-redirects \
		--func-next-line \
			$(SHELL_PROGRAMS)

test: # @HELP Run all tests and output a summary using gotestsum.
test: TFORMAT ?= short
test: GOTEST_FLAGS ?=
test: COMBINED_FLAGS ?= $(GOTEST_FLAGS) $(TEST_PACKAGES)
test: tool-gotestsum
	gotestsum --format=$(TFORMAT) -- $(COMBINED_FLAGS)

test-ci: # @HELP Run all tests in CI with coverage and the race detector.
test-ci:
	MOMA_ENDURO_INTEGRATION_TEST=1 $(MAKE) test GOTEST_FLAGS="-race -coverprofile=covreport -covermode=atomic"

test-race: # @HELP Run all tests with the race detector.
test-race:
	$(MAKE) test GOTEST_FLAGS="-race"

test-tparse: # @HELP Run all tests and output a coverage report using tparse.
test-tparse: tool-tparse
	go test -count=1 -json -cover $(TEST_PACKAGES) | tparse -follow -all -notests

tool-%:
	@go tool bine get $* 1> /dev/null

tools: # @HELP Install all tools managed by bine.
tools:
	go tool bine sync

validate-tilt: # @HELP Validate the Tiltfile and Kubernetes manifests.
	tilt alpha tiltfile-result > /dev/null
