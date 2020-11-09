FROM python:3-alpine as builder
ENV OCTODNS_VERSION=0.9.11
WORKDIR /app
RUN apk add --no-cache \
  --update \
  gcc \
  musl-dev \
  libffi-dev \
  openssl-dev
RUN wget https://github.com/github/octodns/archive/v$OCTODNS_VERSION.tar.gz \
  && ls -alh \
  && tar xvf /app/v$OCTODNS_VERSION.tar.gz \
  && pip install virtualenv \
  && virtualenv /app \
  && source /app/bin/activate \
  && cd octodns-$OCTODNS_VERSION \
  && pip install -r requirements.txt \
  && python setup.py install \
  && cd /app \
  && rm -rf octodns-$OCTODNS_VERSION
FROM python:3-alpine
RUN adduser -D user
RUN apk add --no-cache \
  ca-certificates bash
COPY --from=builder /app /app/
COPY run-job.sh /app/bin/run-job.sh
USER user
ENV PATH /app/bin:$PATH
CMD ["octodns-sync"]
