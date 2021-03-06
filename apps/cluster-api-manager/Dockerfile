FROM golang:1.14.6-alpine3.11 AS api
WORKDIR /app
COPY . /app/
COPY go.* /app/
ARG GOARCH=""
ARG AppBuildVersion="0.0.0"
ARG AppBuildHash="???"
ARG AppBuildDate="???"
ARG AppBuildMode="development"
RUN CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build \
  -a \
  -installsuffix cgo \
  -ldflags "-extldflags '-static' -s -w \
    -X github.com/sharingio/pair/common.AppBuildVersion=$AppBuildVersion \
    -X github.com/sharingio/pair/common.AppBuildHash=$AppBuildHash \
    -X github.com/sharingio/pair/common.AppBuildDate=$AppBuildDate \
    -X github.com/sharingio/pair/common.AppBuildMode=$AppBuildMode" \
  -o cluster-api-manager \
  main.go

FROM alpine:3.11 as extras
RUN apk add tzdata ca-certificates
RUN adduser -D user

FROM scratch
WORKDIR /app
ENV PATH=/app
COPY --from=api /app/cluster-api-manager .
COPY --from=extras /etc/passwd /etc/passwd
COPY --from=extras /etc/group /etc/group
COPY --from=extras /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=extras /etc/ssl /etc/ssl
EXPOSE 8080
USER user
ENTRYPOINT ["/app/cluster-api-manager"]
