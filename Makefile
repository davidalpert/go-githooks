# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

# go versioning flags
VERSION=$(shell cat VERSION)

# ---------------------- targets -------------------------------------

.PHONY: default
default: help

## clean: clean build output
.PHONY: clean
clean:
	rm -rf ./bin

## build: build git hooks
.PHONY: build
build:
	mkdir -p bin/darwin
	go build -ldflags="-X 'main.Version=${VERSION}'" -o bin/darwin/prepare-commit-msg-go-darwin cmd/prepare-commit-msg/main.go

## rebuild: clean and build
.PHONY: rebuild
rebuild: clean build

## test: run tests
.PHONY: test
test:
	#go build -o ../mob-test/.git/hooks/prepare-commit-msg ./cmd/prepare-commit-msg/main.go
	go test -v ./...

## deploy: deploy binaries
.PHONY: deploy
deploy: build
	$(if $(shell which ./deploy.sh),./deploy.sh,$(error "./deploy.sh not found"))

.PHONY: help
help: Makefile
	@echo
	@echo " $(PROJECTNAME) $(VERSION) - available targets:"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo