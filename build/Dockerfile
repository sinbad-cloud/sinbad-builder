FROM janeczku/alpine-kubernetes:3.3

RUN apk --update --no-cache add \
	git \
	ca-certificates
ADD bin/linux/kigo-builder /bin/kigo-builder

ENTRYPOINT ["kigo-builder"]