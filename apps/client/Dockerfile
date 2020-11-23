FROM clojure:openjdk-15-lein-2.9.3-slim-buster as build
WORKDIR /app
COPY src/sharingio /app/src/sharingio
COPY resources /app/resources
COPY test /app/test
COPY *.clj /app/
RUN lein uberjar

FROM gcr.io/distroless/java:11
WORKDIR /app
COPY --from=build /app/target/uberjar/*.jar /app/
ENTRYPOINT ["java", "-cp", "sharingio-standalone.jar", "clojure.main", "-m", "sharingio.web"]
