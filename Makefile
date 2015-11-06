VERSION_VAR := main.VERSION
REPO_VERSION := $(shell git describe --always --dirty --tags)
GOBUILD_VERSION_ARGS := -ldflags "-X $(VERSION_VAR)=$(REPO_VERSION)"

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

setup-cross:
	go get -v -u github.com/laher/goxc

cross:
	goxc -n=rebuild -bc="$(ARCH)" -pv="${REPO_VERSION}" -d=cross

version:
	@echo $(REPO_VERSION)
