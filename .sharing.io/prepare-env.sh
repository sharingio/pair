#!/bin/bash

GIT_ROOT=$(git rev-parse --show-toplevel)

echo "===="
echo "Pair"
echo "===="
echo
echo "# Env set up"
echo "TODO: Navigate to 'https://github.com/settings/developers' -> OAuth Apps"
echo "      go to an existing or new OAuth app."
echo "      ensure that:"
echo "        - homepage URL is set to 'https://pair.${SHARINGIO_PAIR_BASE_DNS_NAME}'"
echo "        - authorization callback URL is set to 'https://pair.${SHARINGIO_PAIR_BASE_DNS_NAME}/oauth'"
echo
echo "Input:"
read -r -p "OAUTH_CLIENT_ID (github oauth app client id)                   : " OAUTH_CLIENT_ID
read -r -p "OAUTH_CLIENT_SECRET (github oauth app client generated secret) : " OAUTH_CLIENT_SECRET
read -r -p "PAIR_PERMITTED_ORGS (github orgs to require for use)           : " PAIR_PERMITTED_ORGS
read -r -p "PAIR_ADMIN_EMAIL_DOMAIN (email address domain for admin access): " PAIR_ADMIN_EMAIL_DOMAIN
echo
echo "Appending to '$GIT_ROOT/.env'"
cat <<EOF >> $GIT_ROOT/.env
OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}
OAUTH_CLIENT_SECRET=${OAUTH_CLIENT_SECRET}
PAIR_PERMITTED_ORGS="${PAIR_PERMITTED_ORGS:-sharingio cncf kubernetes}"
PAIR_ADMIN_EMAIL_DOMAIN=${PAIR_ADMIN_EMAIL_DOMAIN}
EOF
touch $GIT_ROOT/.sharing.io/setup-complete
echo
echo "Please that note that it may take a minute to bring everything up."
echo
echo "Access Pair's client/frontend in development from 'https://pair.${SHARINGIO_PAIR_BASE_DNS_NAME}'"
echo "Access Pair's backend from 'http://localhost:8080/api'"
echo
read -r -p "Press enter to exit"
