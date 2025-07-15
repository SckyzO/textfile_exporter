BINARY_NAME=textfile_exporter
VERSION=0.1.4

# Gather build info
GOVERSION   := $(shell go version)
GIT_COMMIT  := $(shell git rev-parse HEAD)
GIT_DIRTY   := $(shell test -n "`git status --porcelain`" && echo "-dirty")
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD)
BUILD_USER  := $(shell git config user.name) <$(shell git config user.email)> 
BUILD_DATE  := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
PROJECT_URL := https://github.com/SckyzO/textfile_exporter

# Setup ldflags
LDFLAGS = -ldflags "-X 'main.version=$(VERSION)$(GIT_DIRTY)' \
                   -X 'main.revision=$(GIT_COMMIT)' \
                   -X 'main.branch=$(GIT_BRANCH)' \
                   -X 'main.buildUser=$(BUILD_USER)' \
                   -X 'main.buildDate=$(BUILD_DATE)' \
                   -X 'main.goVersion=$(GOVERSION)' \
                   -X 'main.projectURL=$(PROJECT_URL)'"

.PHONY: build
build:
	@echo ">> building $(BINARY_NAME)"
	go build $(LDFLAGS) -o build/$(BINARY_NAME) ./cmd/textfile_exporter

.PHONY: clean
clean:
	@echo ">> cleaning"
	rm -f build/$(BINARY_NAME)

