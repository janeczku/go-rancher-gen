# These env vars have to be set in the CI
# GITHUB_TOKEN
# DOCKER_HUB_TOKEN

.PHONY: build deps test release clean push image ci-compile build-dir ci-dist dist-dir ci-release version help

PROJECT := rancher-gen
PLATFORM := linux
ARCH := amd64
DOCKER_IMAGE := janeczku/$(PROJECT)

VERSION := $(shell cat VERSION)
GITSHA := $(shell git rev-parse --short HEAD)

all: help

help:
	@echo "make build - build binary for the target environment"
	@echo "make deps - install build dependencies"
	@echo "make vet - run vet & gofmt checks"
	@echo "make test - run tests"
	@echo "make clean - Duh!"
	@echo "make release - tag with version and trigger CI release build"
	@echo "make image - build release image"
	@echo "make dev-image - build development image"
	@echo "make dockerhub - build and push image to Docker Hub"
	@echo "make version - show app version"

build: build-dir
	CGO_ENABLED=0 GOOS=$(PLATFORM) GOARCH=$(ARCH) godep go build -ldflags "-X main.Version=$(VERSION) -X main.GitSHA=$(GITSHA)" -o build/$(PROJECT)-$(PLATFORM)-$(ARCH)

deps:
	go get github.com/tools/godep
	go get github.com/c4milo/github-release

vet:
	scripts/vet

test:
	godep go test -v ./...

release:
	git tag `cat VERSION`
	git push origin master --tags

clean:
	go clean
	rm -fr ./build
	rm -fr ./dist

dockerhub: image
	@echo "Pushing $(DOCKER_IMAGE):$(VERSION)"
	docker push $(DOCKER_IMAGE):$(VERSION)

image:
	docker build -t $(DOCKER_IMAGE):$(VERSION) -f Dockerfile .

dev-image:
	docker build -t $(DOCKER_IMAGE):dev -f Dockerfile.dev .

version:
	@echo $(VERSION) $(GITSHA)

ci-compile: build-dir
	CGO_ENABLED=0 GOOS=$(PLATFORM) GOARCH=$(ARCH) godep go build -ldflags "-X main.Version=$(VERSION) -X main.GitSHA=$(GITSHA) -w -s" -a -o build/$(PROJECT)-$(PLATFORM)-$(ARCH)/$(PROJECT)

build-dir:
	@rm -rf build && mkdir build

dist-dir:
	@rm -rf dist && mkdir dist

ci-dist: ci-compile dist-dir
	$(eval FILES := $(shell ls build))
	@for f in $(FILES); do \
		(cd $(shell pwd)/build/$$f && tar -cvzf ../../dist/$$f.tar.gz *); \
		(cd $(shell pwd)/dist && shasum -a 256 $$f.tar.gz > $$f.sha256); \
		(cd $(shell pwd)/dist && md5sum $$f.tar.gz > $$f.md5); \
		echo $$f; \
	done
	@cp -r $(shell pwd)/dist/* $(CIRCLE_ARTIFACTS)
	ls $(CIRCLE_ARTIFACTS)

ci-release:
	@previous_tag=$$(git describe --abbrev=0 --tags $(VERSION)^); \
	comparison="$$previous_tag..HEAD"; \
	if [ -z "$$previous_tag" ]; then comparison=""; fi; \
	changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
	github-release $(CIRCLE_PROJECT_USERNAME)/$(CIRCLE_PROJECT_REPONAME) $(VERSION) master "**Changelog**<br/>$$changelog" 'dist/*'
