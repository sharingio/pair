# Cleanup

# Sometimes there can be quite a lot of left over resources related to instances to don't exist.
# For a healthier system, we are able to clean things up.


LIVE_INSTANCES=$(curl http://sharingio-pair-clusterapimanager.sharingio-pair:8080/api/instance 2>/dev/null | jq -r '.list[].spec.name')
DNSENDPOINTS=$(kubectl -n sharingio-pair get dnsendpoints -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')

DNSENDPOINTS_TO_DELETE=""
for DNSENDPOINT in $DNSENDPOINTS; do
  INSTANCE_NAME=$(kubectl -n sharingio-pair get dnsendpoint $DNSENDPOINT -o=jsonpath="{.metadata.labels."io\\.sharing\\.pair-spec-name"}")
  if echo $LIVE_INSTANCES | grep -q -E "(^| )$INSTANCE_NAME( |$)"; then
    echo "Instance '$INSTANCE_NAME' is alive"
  else
    echo "Instance '$INSTANCE_NAME' not found, removing DNSEndpoint"
    DNSENDPOINTS_TO_DELETE="$DNSENDPOINTS_TO_DELETE $DNSENDPOINT"
  fi
done
if [ ! -z "$DNSENDPOINTS_TO_DELETE" ] ; then
  kubectl -n sharingio-pair delete dnsendpoint $DNSENDPOINTS_TO_DELETE
fi
