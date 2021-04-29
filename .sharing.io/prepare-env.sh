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
echo "        - homepage URL is set to 'https://java.${SHARINGIO_PAIR_BASE_DNS_NAME}'"
echo "        - authorization callback URL is set to 'https://java.${SHARINGIO_PAIR_BASE_DNS_NAME}/oauth'"
read -r -p "OAUTH_CLIENT_ID (github oauth app client id)                  : " OAUTH_CLIENT_ID
read -r -p "OAUTH_CLIENT_SECRET (github oauth app client generated secret): " OAUTH_CLIENT_SECRET

echo "Appending to '$GIT_ROOT/.env'"
cat <<EOF >> $GIT_ROOT/.env
OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}
OAUTH_CLIENT_SECRET=${OAUTH_CLIENT_SECRET}
EOF

touch $GIT_ROOT/.sharing.io/setup-complete

echo "Access Pair in development from 'https://java.${SHARINGIO_PAIR_BASE_DNS_NAME}'"
read -r -p "Press enter to exit"
