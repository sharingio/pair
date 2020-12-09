
#!/usr/bin/env bash

# Installation script "borrowed" from Borkdude's babashka       installation script.
# Consider checking out and/or supporting Borkdude's excellent  work: https://github.com/borkdude/
# then we modified it to work in a netlify build.
set -euo pipefail

latest_release="$(curl -sL https://raw.githubusercontent.com/   theiceshelf/firn/master/clojure/resources/FIRN_VERSION)"

case "$(uname -s)" in
    Linux*)     platform=linux;;
    Darwin*)    platform=mac;;
esac

download_url="https://github.com/theiceshelf/firn/releases/     download/v$latest_release/firn-$platform.zip"
echo -e "Downloading Firn from: $download_url."
curl -o "firn-$latest_release-$platform.zip" -sL $download_url
unzip -qqo "firn-$latest_release-$platform.zip"
chmod +x firn
./firn build
echo "Firn built the site!"
