# Build context is the repository root.
# Generated proto stubs are gitignored, so this image runs `protoc` during the build.

FROM golang:1.25-alpine AS build
RUN apk add --no-cache git make protobuf
WORKDIR /src

COPY go.work go.work.sum ./
COPY proto/gen/go/go.mod proto/gen/go/go.sum ./proto/gen/go/
COPY services/post-service/go.mod services/post-service/go.sum ./services/post-service/
COPY services/feed-service/go.mod services/feed-service/go.sum ./services/feed-service/
COPY services/fanout-worker/go.mod services/fanout-worker/go.sum ./services/fanout-worker/
RUN go mod download

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="/go/bin:${PATH}"

COPY proto ./proto
COPY services ./services
COPY Makefile ./
RUN make proto-go \
    && CGO_ENABLED=0 go build -o /out/post-service ./services/post-service/cmd/server \
    && CGO_ENABLED=0 go build -o /out/feed-service ./services/feed-service/cmd/server \
    && CGO_ENABLED=0 go build -o /out/fanout-worker ./services/fanout-worker/cmd/worker \
    && CGO_ENABLED=0 go build -o /out/warm-cache ./services/fanout-worker/cmd/warm-cache

FROM alpine:3.21 AS post-service
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/post-service /usr/local/bin/post-service
USER nobody
EXPOSE 9090 9100
ENTRYPOINT ["/usr/local/bin/post-service"]

FROM alpine:3.21 AS feed-service
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/feed-service /usr/local/bin/feed-service
USER nobody
EXPOSE 9091 9101
ENTRYPOINT ["/usr/local/bin/feed-service"]

FROM alpine:3.21 AS fanout-worker
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fanout-worker /usr/local/bin/fanout-worker
USER nobody
EXPOSE 9102
ENTRYPOINT ["/usr/local/bin/fanout-worker"]

FROM alpine:3.21 AS warm-cache
RUN apk add --no-cache ca-certificates
COPY --from=build /out/warm-cache /usr/local/bin/warm-cache
USER nobody
ENTRYPOINT ["/usr/local/bin/warm-cache"]
