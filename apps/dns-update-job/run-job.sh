#!/bin/bash

# env
#   - DOMAIN
#   - SUBDOMAIN
#   - ADDRESS
#   - AWS_ACCESS_KEY_ID
#   - AWS_SECRET_ACCESS_KEY
#   - DRYRUN

# REQUIRED_ENV="DOMAIN SUBDOMAIN ADDRESS AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY"
# MISSING_ENV=false
# for ENVVAR in $REQUIRED_ENV; do
#     echo ${!ENVVAR}
#     if [[ -n "${!ENVVAR}" ]]; then
#         echo "[error] missing env ${!ENVVAR}"
#         MISSING_ENV=true
#     fi
# done
# if [ "$MISSING_ENV" = true ]; then
#     exit 1
# fi

mkdir -p /home/user/config

cat <<EOF > /home/user/production.yaml
manager:
  max_workers: 2

providers:
  config:
    class: octodns.provider.yaml.YamlProvider
    directory: /home/user/config
    default_ttl: 3600
    enforce_order: True
  route53:
    class: octodns.provider.route53.Route53Provider
    access_key_id: env/AWS_ACCESS_KEY_ID
    secret_access_key: env/AWS_SECRET_ACCESS_KEY

zones:
  ${DOMAIN}.:
    sources:
      - config
    targets:
      - route53
EOF

cat /home/user/production.yaml

cat <<EOF > "/home/user/config/$DOMAIN.yaml"
'*.${SUBDOMAIN}':
  ttl: 60
  type: A
  values:
  - ${ADDRESS}
'${SUBDOMAIN}':
  ttl: 60
  type: A
  values:
  - ${ADDRESS}
EOF

cat "/home/user/config/$DOMAIN.yaml"

if [ ! "$DRYRUN" = true ]; then
    OPTS="${OPTS} --doit"
fi

octodns-sync --config-file=/home/user/production.yaml $OPTS
