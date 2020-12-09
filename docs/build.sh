#!/usr/bin/env bash
set -euo pipefail

echo "hello, it is me, build."
curl -s https://raw.githubusercontent.com/theiceshelf/firn/master/install -o install-firn
chmod +x install-firn && ./install-firn
firn build
echo "All done!"
