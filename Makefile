GOPATH ?= $(HOME)/go
GOFILES ?= $(shell go list $(TEST) | grep -v /vendor/)
GOTAGS ?=
GOMAXPROCS ?= 4
NAME := $(notdir $(PROJECT))

TEST ?= ./...

# test runs the test suite.
test: fmtcheck
	@echo "==> Testing ${NAME}"
	@go test -v -timeout=300s -parallel=20 -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}
.PHONY: test

# test-race runs the test suite.
test-race: fmtcheck
	@echo "==> Testing ${NAME} (race)"
	@go test -v -timeout=300s -race -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}
.PHONY: test-race

fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -s -w .

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/fmtcheck.sh'"
.PHONY: fmtcheck

