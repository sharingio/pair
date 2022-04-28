FROM golang:1.18.1-alpine3.15 AS api
WORKDIR /app
COPY go.* *.go /app/
ARG GOARCH=""
ARG AppBuildVersion="0.0.0"
ARG AppBuildHash="???"
ARG AppBuildDate="???"
ARG AppBuildMode="development"
RUN CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build \
  -a \
  -installsuffix cgo \
  -ldflags "-extldflags '-static' -s -w \
    -X github.com/sharingio/pair/apps/reconciler.AppBuildVersion=$AppBuildVersion \
    -X github.com/sharingio/pair/apps/reconciler.AppBuildHash=$AppBuildHash \
    -X github.com/sharingio/pair/apps/reconciler.AppBuildDate=$AppBuildDate \
    -X github.com/sharingio/pair/apps/reconciler.AppBuildMode=$AppBuildMode" \
  -o bin/reconciler \
  main.go

FROM alpine:3.15 as extras
RUN apk add tzdata ca-certificates
RUN adduser -D user

FROM scratch
WORKDIR /app
ENV PATH=/app/bin
COPY --from=api /app/bin/reconciler /app/bin/reconciler
COPY --from=extras /etc/passwd /etc/passwd
COPY --from=extras /etc/group /etc/group
COPY --from=extras /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=extras /etc/ssl /etc/ssl
EXPOSE 8080
USER user
ENTRYPOINT ["/app/bin/reconciler"]
