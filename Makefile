APPNAME    = go-git-release
REPOSITORY = $(shell go list -m)
GIT_COMMIT = $(shell git rev-parse --short HEAD)

BUILDFLAGS ?=
LDFLAGS = -ldflags="-X '${REPOSITORY}/cmd.GitCommit=${GIT_COMMIT}'"
unexport GOFLAGS

all: format mod build test

format: vet fmt docs

fmt:
	@echo "gofmt"
	@gofmt -w -s .
	@git diff --exit-code .

build:
	go build ${BUILDFLAGS} ${LDFLAGS} -o ./bin/$(APPNAME) main.go

# build-release is the default make target for go-git-release
# build-release in this instance just does the same as build
build-release: build

vet:
	go vet ${BUILDFLAGS} ./...

mod:
	go mod tidy
	@git diff --exit-code -- go.mod

.PHONY: test
test:
	go test ${BUILDFLAGS} ./... -covermode=atomic -coverpkg=./...

docs:
	@ echo "Placeholder"
