FROM golang:1.10-alpine AS builder

RUN apk update && apk add curl git
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

ADD ./src /go/src/github.com/33arc/raft-kv/src
ADD go.mod /go/src/github.com/33arc/raft-kv/
ADD go.sum  /go/src/github.com/33arc/raft-kv/

WORKDIR /go/src/github.com/33arc/raft-kv/

# RUN dep ensure -vendor-only
RUN go build -o /tmp/raft ./src/*.go


FROM golang:1.23.2-alpine3.20
COPY --from=builder /tmp/raft /app/
CMD /app/raft
