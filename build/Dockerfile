FROM janeczku/alpine-kubernetes:3.3

RUN apk --update --no-cache add \
	git \
	ca-certificates
ADD bin/linux/sinbad-builder /bin/sinbad-builder

ENTRYPOINT ["sinbad-builder"]