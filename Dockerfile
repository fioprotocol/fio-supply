FROM golang:latest AS builder

COPY . /go/src/github.com/fioprotocol/fio-supply/
WORKDIR /go/src/github.com/fioprotocol/fio-supply/
RUN go build -o /fio-supply -ldflags "-s -w" main.go || exit 1

FROM debian:10 AS ssl
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get -y upgrade && apt-get install -y ca-certificates

FROM scratch

# this gets libssl, openssl, and libc6 which we need for TLS, not tiny, but saves about >1 GiB in final image size
COPY --from=ssl /etc/ca-certificates /etc/ca-certificates
COPY --from=ssl /etc/ssl /etc/ssl
COPY --from=ssl /usr/share/ca-certificates /usr/share/ca-certificates
COPY --from=ssl /usr/lib /usr/lib
COPY --from=ssl /lib /lib
COPY --from=ssl /lib64 /lib64

COPY --from=builder /fio-supply /

USER 65535
CMD ["/fio-supply"]
