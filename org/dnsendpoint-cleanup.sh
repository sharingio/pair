# Cleanup

# Sometimes there can be quite a lot of left over resources related to instances to don't exist.
# For a healthier system, we are able to clean things up.


LIVE_INSTANCES=$(curl http://sharingio-pair-clusterapimanager.sharingio-pair:8080/api/instance 2>/dev/null | jq -r '.list[].spec.name')
DNSENDPOINTS=$(kubectl -n sharingio-pair get dnsendpoints | awk '{print $1}' | grep -E '[a-z0-9-]' | xargs -0)

for DNSENDPOINT in $DNSENDPOINTS; do
  kubectl -n sharingio-pair get dnsendpoint $DNSENDPOINT -o=jsonpath='{.metadata.labels}' | jq -r '."io.sharing.pair-spec-name"'
  if echo $LIVE_INSTANCES | grep -q $(kubectl -n sharingio-pair get dnsendpoint $DNSENDPOINT -o=jsonpath='{.metadata.labels}' | jq -r '."io.sharing.pair-spec-name"') ; then
    echo "yes: $DNSENDPOINT"
  else
    echo "no : $DNSENDPOINT"
  fi
done
