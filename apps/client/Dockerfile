FROM alpine:3.14 AS prepare
RUN apk add --no-cache openjdk11 tzdata ca-certificates
RUN adduser -D user

FROM scratch AS base
ENV PATH=/app/bin
COPY --from=prepare /etc/passwd /etc/passwd
COPY --from=prepare /etc/group /etc/group
COPY --from=prepare /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=prepare /etc/ssl /etc/ssl
COPY --from=prepare /etc/ca-certificates /etc/ca-certificates
COPY --from=prepare /etc/ca-certificates.conf /etc/ca-certificates.conf
COPY --from=prepare /usr/bin /usr/bin
COPY --from=prepare /usr/lib /usr/lib
COPY --from=prepare /lib /lib
COPY --from=prepare /usr/share/ca-certificates /usr/share/ca-certificates
USER user

FROM clojure:openjdk-15-lein-2.9.3-slim-buster as build
WORKDIR /app
COPY src/client /app/src/client
COPY resources /app/resources
COPY *.clj /app/
RUN lein ring uberjar

FROM base AS final
ENV PATH=/usr/bin
WORKDIR /app
COPY --from=build /app/target/uberjar/*.jar /app/
ENTRYPOINT ["java", "-jar", "client-0.1.0-standalone.jar"]
