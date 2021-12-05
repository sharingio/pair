package dns

import (
	"context"
	"fmt"
	"log"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	externaldnsendpoint "sigs.k8s.io/external-dns/endpoint"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
)

// Entry ...
// a basic DNS record
type Entry struct {
	Subdomain string   `json:"subdomain"`
	Values    []string `json:"values"`
}

// ReverseDomain ...
// convert domain name into reverse domain name
func ReverseDomain(name string) (output string) {
	nameSplit := strings.Split(name, ".")
	reverseDomainSplit := common.ReverseStringArray(nameSplit)
	reverseDomain := strings.Join(reverseDomainSplit, ".")
	return reverseDomain
}

// FormatAsName ...
// tidy a domain name
func FormatAsName(name string) (output string) {
	nameSplit := strings.Split(name, ".")
	output = strings.Join(nameSplit, "-")
	return output
}

// UpsertDNSEndpoint ...
// create or update (if it already exists) a DNS endpoint (managed by external-dns) in the managed zone
func UpsertDNSEndpoint(dynamicClientset dynamic.Interface, entry Entry, instanceName string) (err error) {
	targetNamespace := common.GetTargetNamespace()

	baseHost := common.GetBaseHost()
	dnsName := entry.Subdomain + "." + baseHost
	dnsNameNS := "ns1." + dnsName
	hostReverse := ReverseDomain(dnsName)
	name := strings.Replace(FormatAsName(hostReverse), "*", "wildcard", -1)
	log.Println("names:", name, dnsName)

	endpoint := externaldnsendpoint.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"io.sharing.pair-spec-name": instanceName,
			},
		},
		Spec: externaldnsendpoint.DNSEndpointSpec{
			Endpoints: []*externaldnsendpoint.Endpoint{
				{
					DNSName:    dnsNameNS,
					Targets:    entry.Values,
					RecordTTL:  60,
					RecordType: "A",
				},
				{
					DNSName:    dnsName,
					Targets:    []string{dnsNameNS},
					RecordTTL:  60,
					RecordType: "NS",
				},
			},
		},
	}
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha1", Group: "externaldns.k8s.io", Resource: "dnsendpoints"}
	asUnstructured, err := common.ObjectToUnstructured(endpoint)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: groupVersionResource.Group, Kind: "DNSEndpoint"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure DNSEndpoint, %#v", err)
	}
	log.Println("attempting create of DNSEndpoint")
	_, err = dynamicClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create DNSEndpoint, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		err = nil
		dnsendpoint, err := dynamicClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			log.Println("%#v\n", err)
			return fmt.Errorf("Failed to get DNSEndpoint (for metadata.resourceVersion), %#v", err)
		}
		asUnstructured.SetResourceVersion(dnsendpoint.GetResourceVersion())
		log.Println("attempting update of DNSEndpoint")
		_, err = dynamicClientset.Resource(groupVersionResource).Namespace(targetNamespace).Update(context.TODO(), asUnstructured, metav1.UpdateOptions{})
		if err != nil {
			log.Println("%#v\n", err)
			return fmt.Errorf("Failed to update DNSEndpoint, %#v", err)
		}
	}
	return err
}
