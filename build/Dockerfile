FROM janeczku/alpine-kubernetes:3.3

RUN apk --update --no-cache add \
	git \
	ca-certificates
ADD bin/kigo-builder-linux /bin/kigo-builder

ENTRYPOINT ["kigo-builder"]