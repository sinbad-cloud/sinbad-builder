FROM gliderlabs/alpine:3.2

RUN apk --update add git ca-certificates
ADD rebuild-linux /rebuild

ENTRYPOINT ["/rebuild"]