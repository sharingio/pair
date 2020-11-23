FROM clojure:openjdk-15-lein-2.9.3-slim-buster as build
WORKDIR /app
COPY src/client /app/src/client
COPY resources /app/resources
COPY test /app/test
COPY *.clj /app/
RUN lein ring uberjar

FROM gcr.io/distroless/java:11
WORKDIR /app
COPY --from=build /app/target/uberjar/*.jar /app/
ENTRYPOINT ["java", "-jar", "client-0.1.0-standalone.jar"]
