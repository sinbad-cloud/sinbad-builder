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
	go build -o rebuild -x $(GOBUILD_VERSION_ARGS) bitbucket.org/jtblin/rebuild

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
	 CGO_ENABLED=0 GOOS=linux go build -ldflags "-s" -a -installsuffix cgo -o rebuild-linux .

docker: cross
	 docker build -t rebuild:$(GIT_HASH) .

version:
	@echo $(REPO_VERSION)
