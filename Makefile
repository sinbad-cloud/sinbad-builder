VERSION_VAR := main.VERSION
REPO_VERSION := $(shell git describe --always --dirty --tags)
GOBUILD_VERSION_ARGS := -ldflags "-X $(VERSION_VAR)=$(REPO_VERSION)"
GIT_HASH := $(shell git rev-parse --short HEAD)

ARCH := linux darwin windows freebsd

setup:
	go get -v
	go get -v -u github.com/githubnemo/CompileDaemon
	go get -v -u github.com/alecthomas/gometalinter
	gometalinter --install --update

build: *.go
	gofmt -w=true .
	goimports -w=true .
	go build -o build/bin/kigo-builder -x $(GOBUILD_VERSION_ARGS) bitbucket.org/jtblin/kigo-builder

test: build
	go test

junit-test: build
	go get github.com/jstemmer/go-junit-report
	go test -v | go-junit-report > test-report.xml

check: build
	gometalinter ./...

watch:
	CompileDaemon -color=true -build "make test check"

commit-hook:
	cp dev/commit-hook.sh .git/hooks/pre-commit

cross:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" -a -installsuffix cgo -o build/bin/kigo-builder-linux .

docker: cross
	cd build && docker build -t jtblin/kigo-builder:$(GIT_HASH) .

release: docker
	docker push jtblin/kigo-builder:$(GIT_HASH)
	docker tag -f jtblin/kigo-builder:$(GIT_HASH) jtblin/kigo-builder:latest
	docker push jtblin/kigo-builder:latest

version:
	@echo $(REPO_VERSION)

clean:
	-docker rm $(docker ps -a -f 'status=exited' -q)
	-docker rmi $(docker images -f 'dangling=true' -q)

herokuish:
	cd build && docker build -t jtblin/herokuish:0.3.5 -f Dockerfile.herokuish .

.PHONY: build
