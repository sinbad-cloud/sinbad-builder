BINARY_NAME := sinbad-builder
ORG_PATH="github.com/sinbad-cloud"
REPO_PATH="$(ORG_PATH)/$(BINARY_NAME)"
VERSION_VAR := $(REPO_PATH)/version.Version
GIT_VAR := $(REPO_PATH)/version.GitCommit
BUILD_DATE_VAR := $(REPO_PATH)/version.BuildDate
REPO_VERSION := $$(git describe --abbrev=0 --tags)
BUILD_DATE := $$(date +%Y-%m-%d-%H:%M)
GIT_HASH := $$(git rev-parse --short HEAD)
GOBUILD_VERSION_ARGS := -ldflags "-s -X $(VERSION_VAR)=$(REPO_VERSION) -X $(GIT_VAR)=$(GIT_HASH) -X $(BUILD_DATE_VAR)=$(BUILD_DATE)"
IMAGE_NAME := sinbad/$(BINARY_NAME)
ARCH ?= darwin

setup:
	go get -v -u github.com/Masterminds/glide
	go get -v -u github.com/githubnemo/CompileDaemon
	go get -v -u github.com/alecthomas/gometalinter
	go get -v -u github.com/jstemmer/go-junit-report
	gometalinter --install --update
	glide install

build: *.go fmt
	go build -o build/bin/$(ARCH)/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) $(REPO_PATH)

fmt:
	gofmt -w=true -s $$(find . -type f -name '*.go' -not -path "./vendor/*")
	goimports -w=true -d $$(find . -type f -name '*.go' -not -path "./vendor/*")

test:
	go test $$(glide nv)

race:
	go build -race $$(glide nv)

bench:
	go test -bench=. $$(glide nv)

cover:
	./cover.sh

junit-test: build
	go test -v $(glide nv) | go-junit-report > test-report.xml

check:
	go install
	gometalinter --deadline=60s ./... --vendor --linter='errcheck:errcheck:-ignore=net:Close' --cyclo-over=20 --linter='vet:go tool vet -composites=false {paths}:PATH:LINE:MESSAGE'

profile:
	./build/bin/$(ARCH)/$(BINARY_NAME) --backends=stdout --cpu-profile=./profile.out --flush-interval=1s
	go tool pprof build/bin/$(ARCH)/$(BINARY_NAME) profile.out

watch:
	CompileDaemon -color=true -build "make test check"

cross:
	CGO_ENABLED=0 GOOS=linux go build -o build/bin/linux/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) -a -installsuffix cgo  $(REPO_PATH)

docker: cross
	cd build && docker build -t $(IMAGE_NAME):$(GIT_HASH) .

release: test docker
	docker push $(IMAGE_NAME):$(GIT_HASH)
	docker tag -f $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):latest
	docker tag -f $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):$(REPO_VERSION)
	docker push $(IMAGE_NAME):$(REPO_VERSION)

run: build
	./build/bin/$(ARCH)/$(BINARY_NAME) # TODO complete

version:
	@echo $(REPO_VERSION)

clean:
	rm -f build/bin/*
	-docker rm $(docker ps -a -f 'status=exited' -q)
	-docker rmi $(docker images -f 'dangling=true' -q)

.PHONY: build
