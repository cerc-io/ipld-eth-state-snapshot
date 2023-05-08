FROM golang:1.19-alpine as builder

RUN apk --update --no-cache add make git g++ linux-headers
# DEBUG
RUN apk add busybox-extras

# Get and build ipfs-blockchain-watcher
ADD . /go/src/github.com/cerc-io/ipld-eth-state-snapshot
#RUN git clone https://github.com/cerc-io/ipld-eth-state-snapshot.git /go/src/github.com/vulcanize/ipld-eth-state-snapshot

WORKDIR /go/src/github.com/cerc-io/ipld-eth-state-snapshot
RUN GO111MODULE=on GCO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ipld-eth-state-snapshot .

# app container
FROM alpine

RUN apk --no-cache add su-exec bash

WORKDIR /app

COPY --from=builder /go/src/github.com/cerc-io/ipld-eth-state-snapshot/startup_script.sh .
COPY --from=builder /go/src/github.com/cerc-io/ipld-eth-state-snapshot/environments environments

# keep binaries immutable
COPY --from=builder /go/src/github.com/cerc-io/ipld-eth-state-snapshot/ipld-eth-state-snapshot ipld-eth-state-snapshot

ENTRYPOINT ["/app/startup_script.sh"]
