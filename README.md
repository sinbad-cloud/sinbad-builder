# kigo-builder

Build docker containers herokuish style

## Devel

Start an app

    docker run -it node-hello /start web

Run locally (OSX)

    ./build/bin/kigo-builder --namespace=jtblin --repo=node-hello --origin=github.com --verbose \
    	--dir=/Users/jtblin/src/private/node-hello/ --registry=jtblin --docker-machine

Run inside a docker container

    make docker
    docker run --rm -it -v/Users/jtblin/src/private/node-hello:/src -v /var/run/docker.sock:/var/run/docker.sock \
    	-v /Users/jtblin/.docker/config.json:/root/.docker/config.json kigo-builder:<replaceme> \
    	--namespace=jtblin --repo=node-hello --origin=github.com --verbose --registry=jtblin --dir=/src

## Repos for testing

* Dockerfile: https://github.com/enokd/docker-node-hello/
* Dockerfile: https://github.com/shekhargulati/python-flask-docker-hello-world
* Node: https://github.com/jtblin/node-hello
* Python: https://github.com/IBM-Bluemix/python-hello-world-flask