# rebuild

Build docker containers herokuish style

## Devel

Start an app

    docker run -it node-hello /start web

Run locally

    ./rebuild --namespace=jtblin --repo=node-hello --origin=github.com --verbose \
    	--build-step=`pwd`/include/buildstep.sh --dir=/Users/jtblin/.tmp-scripts\
    	 --commit=9a9b307cc0f4dbc461b457719f8ac854f2ca3666 --registry=jtblin

Run inside a docker container

    make docker
    docker run --rm -it -v /Users/jtblin/.tmp-scripts:/src -v /var/run/docker.sock:/var/run/docker.sock \
    	-v /Users/jtblin/.docker/config.json:/root/.docker/config.json -v $(which docker):/bin/docker rebuild:f9802da \
    	--namespace=jtblin --repo=node-hello --origin=github.com --verbose --build-step=/include/buildstep.sh \
    	--commit=9a9b307cc0f4dbc461b457719f8ac854f2ca3666 --registry=jtblin --dir=/src
