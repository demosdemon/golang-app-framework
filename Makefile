SOURCE_FILES = $(shell find . -type f -not -path './vendor/*' -iname '*.go' -not -iname '*_test.go')
TEST_FILES = $(shell find . -type f -not -path './vendor/*' -iname '*_test.go')

_prefix = github.com/demosdemon/golang-app-framework/v0
COMMANDS = $(notdir $(wildcard cmd/*))
PACKAGES = app $(foreach b,$(COMMANDS),cmd/$(b))
BUILD_TARGETS = $(foreach b,$(COMMANDS),build/$(b))
TEST_PACKAGES = $(foreach b,$(PACKAGES),$(_prefix)/$(b))

LDFLAGS = -s -w -extldflags "-static"

.PHONY: help
help:
	@echo ' SOURCE_FILES = $(SOURCE_FILES)'
	@echo '   TEST_FILES = $(TEST_FILES)'
	@echo '      _prefix = $(_prefix)'
	@echo '     COMMANDS = $(COMMANDS)'
	@echo '     PACKAGES = $(PACKAGES)'
	@echo 'BUILD_TARGETS = $(BUILD_TARGETS)'
	@echo 'TEST_PACKAGES = $(TEST_PACKAGES)'
	@echo '      LDFLAGS = $(LDFLAGS)'

.PHONY: all
all: build

.PHONY: build
build: lint $(BUILD_TARGETS)

build/%: $(SOURCE_FILES)
	go build -o $@ -v -a -ldflags '$(LDFLAGS)' $(_prefix)/cmd/$*

.PHONY: debug
debug:
	@$(MAKE) LDFLAGS= build

.PHONY: format
format:
	goreturns -b -i -w $(SOURCE_FILES) $(TEST_FILES)

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test: build
	@set -x; for pkg in $(TEST_PACKAGES); do mkdir -vp build/$$pkg; go test -v -timeout 30s -covermode atomic -coverprofile build/$$pkg/cover.out -trace build/$$pkg/trace.out -coverpkg ./... $$pkg; done
	ls -lhAR build
	gocovmerge $(foreach pkg,$(TEST_PACKAGES),build/$(pkg)/cover.out) > coverage.out

.PHONY: bench
bench:
	go test -v -benchmem -bench=. $(TEST_PACKAGES)
