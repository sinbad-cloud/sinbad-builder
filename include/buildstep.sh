#!/bin/bash
set -eo pipefail

NAME="$1"
TAG="$2"

# Place the app inside the container. If you already have app inside your tar, use /bin/bash -c "tar -x"
ID=$(tar cC . . | docker run -i -a stdin progrium/buildstep /bin/bash -c "mkdir -p /app && tar -xC /app")
test $(docker wait $ID) -eq 0

# Run the builder script and attach to view output
if [[ -z "$TAG" ]]; then
	IMAGE=$NAME
else
	IMAGE=$NAME:$TAG
fi
docker commit $ID $IMAGE > /dev/null
ID=$(docker run -d $IMAGE /build/builder)
docker attach $ID
test $(docker wait $ID) -eq 0
docker commit $ID $IMAGE > /dev/null
docker rm $(docker ps -a -f 'status=exited' -q) > /dev/null
docker rmi $(docker images -f 'dangling=true' -q) > /dev/null
