FROM gliderlabs/alpine:3.2

RUN apk --update add \
	git \
	ca-certificates \
	&& rm -rf /var/cache/apk/*
ADD bin/rebuild-linux /bin/rebuild

ENTRYPOINT ["rebuild"]