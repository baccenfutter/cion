FROM golang:alpine as builder
RUN apk add git
ADD . /go/src/github.com/baccenfutter/cion
WORKDIR /go/src/github.com/baccenfutter/cion
RUN go get github.com/kardianos/govendor
RUN govendor sync
RUN govendor install +local


FROM alpine:latest
LABEL maintainer Brian Wiborg <baccenfutter@c-base.org>

VOLUME /etc/bind/keys
VOLUME /var/bind/dyn

RUN apk add --no-cache bash bind bind-tools sudo

EXPOSE 80/tcp
EXPOSE 53/udp
EXPOSE 53/tcp

WORKDIR /etc/bind

COPY docker/etc/bind/*.conf /etc/bind/
COPY docker/etc/bind/named.conf.default-zones /etc/bind/
COPY docker/etc/bind/db.* /etc/bind/
COPY docker/scripts /docker

COPY --from=builder /go/bin/cion /usr/bin/cion
VOLUME /public

ENV PATH=$PATH:/docker
CMD ["run.sh"]
