# Stage 1
FROM golang:1.19 AS builder

WORKDIR /go/src/github.com/
COPY . oval
WORKDIR /go/src/github.com/oval
RUN make

# Stage 2
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /go/src/github.com/oval/oval ./
ENTRYPOINT [ "./oval" ]
