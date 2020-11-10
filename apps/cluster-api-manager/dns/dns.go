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

	"github.com/sharingio/pair/common"
)

type Entry struct {
	Subdomain string   `json:"subdomain"`
	Values    []string `json:"values"`
}

func ReverseDomain(name string) (output string) {
	nameSplit := strings.Split(name, ".")
	reverseDomainSplit := common.ReverseStringArray(nameSplit)
	reverseDomain := strings.Join(reverseDomainSplit, ".")
	return reverseDomain
}

func FormatAsName(name string) (output string) {
	nameSplit := strings.Split(name, ".")
	reverseDomain := strings.Join(nameSplit, "-")
	return reverseDomain
}

func UpsertDNSEndpoint(dynamicClientset dynamic.Interface, entry Entry) (err error) {
	targetNamespace := common.GetTargetNamespace()

	baseHost := common.GetBaseHost()
	baseHostReverse := ReverseDomain(baseHost)
	name := FormatAsName(baseHostReverse + "." + entry.Subdomain)
	dnsName := entry.Subdomain + "." + baseHost
	log.Println(name, dnsName)
	return err

	endpoint := externaldnsendpoint.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
		Spec: externaldnsendpoint.DNSEndpointSpec{
			Endpoints: []*externaldnsendpoint.Endpoint{
				{
					DNSName:    dnsName,
					Targets:    entry.Values,
					RecordTTL:  60,
					RecordType: "A",
				},
			},
		},
	}
	groupVersionResource := schema.GroupVersionResource{Version: "alphav1", Group: "externaldns.k8s.io", Resource: "dnsendpoints"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured := common.ObjectToUnstructured(endpoint)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: groupVersionResource.Group, Kind: "DNSEndpoint"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure DNSEndpoint, %#v", err)
	}
	_, err = dynamicClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create DNSEndpoint, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		_, err = dynamicClientset.Resource(groupVersionResource).Namespace(targetNamespace).Update(context.TODO(), asUnstructured, metav1.UpdateOptions{})
	}
	return err
}
