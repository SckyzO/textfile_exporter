BINARY_NAME=textfile-exporter
VERSION=0.1

# Gather build info
GOVERSION   := $(shell go version)
GIT_COMMIT  := $(shell git rev-parse HEAD)
GIT_DIRTY   := $(shell test -n "`git status --porcelain`" && echo "-dirty")
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD)
BUILD_USER  := $(shell git config user.name) <$(shell git config user.email)>
BUILD_DATE  := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Setup ldflags
LDFLAGS = -ldflags "-X 'main.version=$(VERSION)$(GIT_DIRTY)' \
                   -X 'main.revision=$(GIT_COMMIT)' \
                   -X 'main.branch=$(GIT_BRANCH)' \
                   -X 'main.buildUser=$(BUILD_USER)' \
                   -X 'main.buildDate=$(BUILD_DATE)' \
                   -X 'main.goVersion=$(GOVERSION)'"

.PHONY: build
build:
	@echo ">> building $(BINARY_NAME)"
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/textfile-exporter

.PHONY: clean
clean:
	@echo ">> cleaning"
	rm -f $(BINARY_NAME)

