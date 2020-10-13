package instances

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"text/template"

	"github.com/sharingio/pair/src/cluster-api-manager/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"

	clusterAPIPacketv1alpha3 "sigs.k8s.io/cluster-api-provider-packet/api/v1alpha3"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

type KubernetesCluster struct {
	KubeadmControlPlane         clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
	Cluster                     clusterAPIv1alpha3.Cluster
	MachineDeploymentWorker     clusterAPIv1alpha3.MachineDeployment
	KubeadmConfigTemplateWorker cabpkv1.KubeadmConfigTemplate
	PacketMachineTemplate       clusterAPIPacketv1alpha3.PacketMachineTemplate
	PacketCluster               clusterAPIPacketv1alpha3.PacketCluster
	PacketMachineTemplateWorker clusterAPIPacketv1alpha3.PacketMachineTemplate
}

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
	TTY                bool
}

func Int32ToInt32Pointer(input int32) *int32 {
	return &input
}

var defaultMachineOS = "ubuntu_20_04"
var defaultKubernetesVersion = "1.19.0"
var defaultKubernetesClusterConfig = KubernetesCluster{
	KubeadmControlPlane: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneSpec{
			Version:  defaultKubernetesVersion,
			Replicas: Int32ToInt32Pointer(1),
			InfrastructureTemplate: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
				Kind:       "PacketMachineTemplate",
			},
			KubeadmConfigSpec: cabpkv1.KubeadmConfigSpec{
				InitConfiguration: &kubeadmv1beta1.InitConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
					},
				},
				ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
					APIServer: kubeadmv1beta1.APIServer{
						ControlPlaneComponent: kubeadmv1beta1.ControlPlaneComponent{
							ExtraArgs: map[string]string{
								"cloud-provider":            "external",
								"audit-policy-file":         "/etc/kubernetes/pki/audit-policy.yaml",
								"audit-log-path":            "-",
								"audit-webhook-config-file": "/etc/kubernetes/pki/audit-sink.yaml",
								"v":                         "99",
							},
						},
					},
					ControllerManager: kubeadmv1beta1.ControlPlaneComponent{
						ExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
					},
				},
				JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "external",
						},
					},
				},
				PreKubeadmCommands: []string{
					"mkdir -p /etc/kubernetes/pki",
					`cat <<EOF > /etc/kubernetes/pki/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: RequestResponse
EOF`,
					`cat <<EOF > /etc/kubernetes/pki/audit-sink.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://10.96.96.96:9900/events
  name: auditsink-cluster
contexts:
- context:
    cluster: auditsink-cluster
    user: ""
  name: auditsink-context
current-context: auditsink-context
users: []
preferences: {}
EOF`,
					"sed -ri '/\\sswap\\s/s/^#?/#/' /etc/fstab",
					"swapoff -a",
					"mount -a",
					"apt-get -y update",
					"DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https curl",
					"curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -",
					"echo \"deb https://apt.kubernetes.io/ kubernetes-xenial main\" > /etc/apt/sources.list.d/kubernetes.list",
					"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -",
					"apt-key fingerprint 0EBFCD88",
					"add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"",
					"apt-get update -y",
					"apt-get install -y ca-certificates socat jq ebtables apt-transport-https cloud-utils prips docker-ce docker-ce-cli containerd.io kubelet kubeadm kubectl",
					"systemctl daemon-reload",
					"systemctl enable docker",
					"systemctl start docker",
					"chgrp users /var/run/docker.sock",
					"ping -c 3 -q {{ .controlPlaneEndpoint }} && echo OK || ip addr add {{ .controlPlaneEndpoint }} dev lo",
				},
				PostKubeadmCommands: []string{
					`cat <<EOF >> /etc/network/interfaces
auto lo:0
iface lo:0 inet static
  address {{ .controlPlaneEndpoint }}
  netmask 255.255.255.255
EOF
`,
					"systemctl restart networking",
					"mkdir -p /root/.kube",
					"cp -i /etc/kubernetes/admin.conf /root/.kube/config",
					"export KUBECONFIG=/root/.kube/config",
					"kubectl taint node --all node-role.kubernetes.io/master-",
					"kubectl create secret generic -n kube-system packet-cloud-config --from-literal=cloud-sa.json='{\"apiKey\": \"{{ .apiKey }}\",\"projectID\": \"%s\", \"eipTag\": \"cluster-api-provider-packet:cluster-id:%s\"}'",
					"kubectl taint node --all node-role.kubernetes.io/master-",
					"kubectl apply -f https://github.com/packethost/packet-ccm/releases/download/v1.1.0/deployment.yaml",
					"kubectl apply -f https://github.com/packethost/csi-packet/raw/master/deploy/kubernetes/setup.yaml",
					"kubectl apply -f https://github.com/packethost/csi-packet/raw/master/deploy/kubernetes/setup.yaml",
					"kubectl apply -f https://github.com/packethost/csi-packet/raw/master/deploy/kubernetes/controller.yaml",
					"kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.1/cert-manager.yaml",
					"kubectl apply -f \"https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')&env.IPALLOC_RANGE=192.168.0.0/16\"",
					"curl -L https://get.helm.sh/helm-v3.3.0-linux-amd64.tar.gz | tar --directory /usr/local/bin --extract -xz --strip-components 1 linux-amd64/helm",
					`(
          helm repo add nginx-ingress https://kubernetes.github.io/ingress-nginx;
          kubectl create ns nginx-ingress;
          helm install nginx-ingress -n nginx-ingress nginx-ingress/ingress-nginx --set controller.service.externalTrafficPolicy=Local --version 2.16.0;
          kubectl wait -n nginx-ingress --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s
        )
`,
					"kubectl get configmap kube-proxy -n kube-system -o yaml | sed -e \"s/strictARP: false/strictARP: true/\" | kubectl apply -f - -n kube-system",
					`cat <<EOF > /root/metallb-system-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
      - name: default
        protocol: layer2
        addresses:
          - {{ .controlPlaneEndpoint }}/32
EOF`,
					`(
          kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml;
          kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml;
          kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)";
          kubectl apply -f /root/metallb-system-config.yaml
        )
`,
					`(
          set -x
          cd /root;
          git clone https://github.com/cncf/apisnoop;
          cd apisnoop;
          kubectl create ns apisnoop;
          helm install snoopdb -n apisnoop charts/snoopdb;
          helm install auditlogger -n apisnoop charts/auditlogger
        )
`,
					// 1,2,3 = cluster name
					// 4 = timezone
					`(
          set -x;
          cd /root;
          git clone https://github.com/humacs/humacs;
          cd humacs;
          kubectl create ns %s
          helm install "%s" -n "%s" -f chart/humacs/values/apisnoop.yaml --set options.timezone="%s" {{ range $index, $repo := $.Repos }}--set options.repos[{{ $index }}]={{ $repo }} {{ end }} chart/humacs
        )
`,
					`(
          mkdir -p /etc/sudoers.d
          echo "%sudo    ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers.d/sudo
          cp -a /root/.ssh /etc/skel/.ssh
          useradd -m -G users,sudo -u 1000 -s /bin/bash ii
        )
`,
				},
			},
		},
	},
	Cluster: clusterAPIv1alpha3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: clusterAPIv1alpha3.ClusterSpec{
			ClusterNetwork: &clusterAPIv1alpha3.ClusterNetwork{
				Pods: &clusterAPIv1alpha3.NetworkRanges{
					CIDRBlocks: []string{
						"10.244.0.0/16",
					},
				},
				Services: &clusterAPIv1alpha3.NetworkRanges{
					CIDRBlocks: []string{
						"10.96.0.0/12",
					},
				},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
				Kind:       "PacketCluster",
			},
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
				Kind:       "KubeadmControlPlane",
			},
		},
	},
	MachineDeploymentWorker: clusterAPIv1alpha3.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
			Labels: map[string]string{
				"pool":            "worker-a",
				"io.sharing.pair": "instance",
			},
		},
		Spec: clusterAPIv1alpha3.MachineDeploymentSpec{
			Replicas:    Int32ToInt32Pointer(0),
			ClusterName: "",
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pool": "worker-a",
				},
			},
			Template: clusterAPIv1alpha3.MachineTemplateSpec{
				ObjectMeta: clusterAPIv1alpha3.ObjectMeta{
					Name: "",
					Labels: map[string]string{
						"io.sharing.pair": "instance",
						"pool":            "worker-a",
					},
				},
				Spec: clusterAPIv1alpha3.MachineSpec{
					Version:     &defaultKubernetesVersion,
					ClusterName: "",
					Bootstrap: clusterAPIv1alpha3.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha3",
							Kind:       "KubeadmConfigTemplate",
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Kind:       "PacketMachineTemplate",
					},
				},
			},
		},
	},
	KubeadmConfigTemplateWorker: cabpkv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: cabpkv1.KubeadmConfigTemplateSpec{
			Template: cabpkv1.KubeadmConfigTemplateResource{
				Spec: cabpkv1.KubeadmConfigSpec{
					PreKubeadmCommands: []string{
						"sed -ri '/\\sswap\\s/s/^#?/#/' /etc/fstab",
						"swapoff -a",
						"mount -a",
						"apt-get -y update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https curl",
						"curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -",
						"echo \"deb https://apt.kubernetes.io/ kubernetes-xenial main\" > /etc/apt/sources.list.d/kubernetes.list",
						"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -",
						"apt-key fingerprint 0EBFCD88",
						"add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"",
						"apt-get update -y",
						"apt-get install -y ca-certificates socat jq ebtables apt-transport-https cloud-utils prips docker-ce docker-ce-cli containerd.io kubelet kubeadm kubectl",
						"systemctl daemon-reload",
						"systemctl enable docker",
						"systemctl start docker",
						"chgrp users /var/run/docker.sock",
					},
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							KubeletExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
						},
					},
				},
			},
		},
	},
	PacketMachineTemplate: clusterAPIPacketv1alpha3.PacketMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
			Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
				Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
					OS:           defaultMachineOS,
					BillingCycle: "hourly",
					// 1 = machine type
					MachineType: "",
				},
			},
		},
	},
	PacketCluster: clusterAPIPacketv1alpha3.PacketCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: clusterAPIPacketv1alpha3.PacketClusterSpec{},
	},
	PacketMachineTemplateWorker: clusterAPIPacketv1alpha3.PacketMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "",
			Labels: map[string]string{"io.sharing.pair": "instance"},
		},
		Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
			Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
				Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
					OS:           defaultMachineOS,
					BillingCycle: "hourly",
					// 1 = machine type
					MachineType: "",
				},
			},
		},
	},
}

func KubernetesGet(name string, kubernetesClientset dynamic.Interface) (err error, instance Instance) {
	targetNamespace := common.GetTargetNamespace()

	// manifests

	//   - newInstance.MachineDeploymentWorker
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	item, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to get Instance (MachineDeployment), %#v", err), instance
	}
	var itemRestructuredMD clusterAPIv1alpha3.MachineDeployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredMD)
	if err != nil {
		return fmt.Errorf("Failed to restructure %T", itemRestructuredMD), Instance{}
	}
	if itemRestructuredMD.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredMD, itemRestructuredMD.ObjectMeta.Name)
	} else {
		instance.Status.Resources.MachineDeploymentWorker = itemRestructuredMD.Status
	}

	//   - newInstance.KubeadmControlPlane
	groupVersionResource = schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	item, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-control-plane", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to Instance (KubeadmControlPlane), %#v", err), instance
	}
	var itemRestructuredKCP clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredKCP)
	if err != nil {
		return fmt.Errorf("Failed to restructure %T", itemRestructuredKCP), Instance{}
	}
	if itemRestructuredKCP.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredKCP, itemRestructuredKCP.ObjectMeta.Name)
	} else {
		instance.Status.Resources.KubeadmControlPlane = itemRestructuredKCP.Status
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	item, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketCluster, %#v", err), instance
	}
	var itemRestructuredPC clusterAPIPacketv1alpha3.PacketCluster
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredPC)
	if err != nil {
		return fmt.Errorf("Failed to restructure %T", itemRestructuredPC), Instance{}
	}
	if itemRestructuredPC.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredPC, itemRestructuredPC.ObjectMeta.Name)
	} else {
		instance.Status.Resources.PacketCluster = itemRestructuredPC.Status
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	item, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create Cluster, %#v", err), instance
	}
	var itemRestructuredC clusterAPIv1alpha3.Cluster
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredC)
	if err != nil {
		return fmt.Errorf("Failed to restructure %T", itemRestructuredC), Instance{}
	}
	if itemRestructuredC.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredC, itemRestructuredC.ObjectMeta.Name)
	} else {
		instance.Status.Resources.Cluster = itemRestructuredC.Status
	}
	instance.Spec.Name = itemRestructuredC.ObjectMeta.Labels["io.sharing.pair-instance"]
	err = nil
	return err, instance
}

func KubernetesList(kubernetesClientset dynamic.Interface, options InstanceListOptions) (err error, instances []Instance) {
	targetNamespace := common.GetTargetNamespace()

	// manifests

	//   - newInstance.MachineDeploymentWorker
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	items, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create MachineDeployment, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.MachineDeployment
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Labels["io.sharing.pair-username"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		var instance = Instance{
			Spec: InstanceSpec{
				Name: itemRestructured.ObjectMeta.Labels["io.sharing.pair-instance"],
			},
			Status: InstanceStatus{
				Resources: InstanceResourceStatus{
					MachineDeploymentWorker: itemRestructured.Status,
				},
			},
		}
		instances = append(instances, instance)
	}

	//   - newInstance.KubeadmControlPlane
	groupVersionResource = schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list KubeadmControlPlane, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Labels["io.sharing.pair-username"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
	instances:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["io.sharing.pair-instance"] {
				instances[i].Status.Resources.KubeadmControlPlane = itemRestructured.Status
				break instances
			}
		}
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketCluster, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIPacketv1alpha3.PacketCluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Labels["io.sharing.pair-username"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
	instances1:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["io.sharing.pair-instance"] {
				instances[i].Status.Resources.PacketCluster = itemRestructured.Status
				break instances1
			}
		}
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create Cluster, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Labels["io.sharing.pair-username"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
	instances2:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["io.sharing.pair-instance"] {
				instances[i].Status.Resources.Cluster = itemRestructured.Status
				break instances2
			}
		}
	}
	err = nil
	return err, instances
}

func KubernetesCreate(instance InstanceSpec, kubernetesClientset dynamic.Interface) (err error, instanceCreated InstanceSpec) {
	// generate name
	targetNamespace := common.GetTargetNamespace()
	err, newInstance := KubernetesTemplateResources(instance, targetNamespace)
	if err != nil {
		return err, instanceCreated
	}
	instanceCreated.Name = instance.Name

	// manifests
	//   - newInstance.KubeadmControlPlane
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured := common.ObjectToUnstructured(newInstance.KubeadmControlPlane)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Kind: "KubeadmControlPlane"})
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create KubeadmControlPlane, %#v", err), instanceCreated
	}

	//   - newInstance.PacketMachineTemplate
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketMachineTemplate)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketMachineTemplate, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachineTemplate, %#v", err), instanceCreated
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketCluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketCluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketCluster, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketCluster, %#v", err), instanceCreated
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.Cluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "Cluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure Cluster, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create Cluster, %#v", err), instanceCreated
	}

	//   - newInstance.MachineDeploymentWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.MachineDeploymentWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "MachineDeployment"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure MachineDeployment, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create MachineDeployment, %#v", err), instanceCreated
	}

	//   - newInstance.KubeadmConfigTemplateWorker
	groupVersion = cabpkv1.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "bootstrap.cluster.x-k8s.io", Resource: "kubeadmconfigtemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.KubeadmConfigTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: groupVersionResource.Group, Kind: "KubeadmConfigTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure KubeadmConfigTemplate, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create KubeadmConfigTemplate, %#v", err), instanceCreated
	}

	//   - newInstance.PacketMachineTemplateWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketMachineTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketMachineTemplateWorker, %#v", err), instanceCreated
	}
	_, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachineTemplateWorker, %#v", err), instanceCreated
	}
	err = nil

	return err, instanceCreated
}

func KubernetesUpdate(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {
	return err, instanceUpdated
}

func KubernetesDelete(name string, kubernetesClientset dynamic.Interface) (err error) {
	// generate name
	targetNamespace := common.GetTargetNamespace()

	// manifests

	//   - newInstance.PacketMachineTemplate
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete PacketMachineTemplate, %#v", err)
	}

	//   - newInstance.KubeadmConfigTemplateWorker
	groupVersion = cabpkv1.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "bootstrap.cluster.x-k8s.io", Resource: "kubeadmconfigtemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create KubeadmConfigTemplate, %#v", err)
	}

	//   - newInstance.PacketMachineTemplateWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachineTemplateWorker, %#v", err)
	}

	//   - newInstance.PacketMachine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachine"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachine, %#v", err)
	}

	//   - newInstance.MachineDeploymentWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create MachineDeployment, %#v", err)
	}

	//   - newInstance.KubeadmControlPlane
	groupVersionResource = schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-control-plane", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete KubeadmControlPlane, %#v", err)
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketCluster, %#v", err)
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create Cluster, %#v", err)
	}
	err = nil

	return err
}

func KubernetesTemplateResources(instance InstanceSpec, namespace string) (err error, newInstance KubernetesCluster) {
	newInstance = defaultKubernetesClusterConfig
	newInstance.KubeadmControlPlane.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.ObjectMeta.Namespace = namespace
	newInstance.KubeadmControlPlane.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.KubeadmControlPlane.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	newInstance.KubeadmControlPlane.Spec.InfrastructureTemplate.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[6] = fmt.Sprintf(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[6], common.GetPacketProjectID(), instance.Name)
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[20] = fmt.Sprintf(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[20], instance.Name, instance.Name, instance.Name, instance.Setup.Timezone)
	fmt.Println(newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[20])
	tmpl, err := template.New("humacs-helm-install").Parse(newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[20])
	if err != nil {
		return fmt.Errorf("Error templating Humacs Helm install command: %#v", err), newInstance
	}
	templatedHumacsHelmInstall := new(bytes.Buffer)
	err = tmpl.Execute(templatedHumacsHelmInstall, instance.Setup)
	if err != nil {
		return fmt.Errorf("Error templating Humacs Helm install command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[20] = templatedHumacsHelmInstall.String()

	newInstance.PacketMachineTemplate.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.PacketMachineTemplate.ObjectMeta.Namespace = namespace
	newInstance.PacketMachineTemplate.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.PacketMachineTemplate.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplate.Spec.Template.Spec.MachineType = "c1.small.x86"

	newInstance.MachineDeploymentWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.ObjectMeta.Namespace = namespace
	newInstance.MachineDeploymentWorker.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.MachineDeploymentWorker.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	newInstance.MachineDeploymentWorker.Spec.ClusterName = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.Bootstrap.ConfigRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Selector.MatchLabels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.InfrastructureRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.ClusterName = instance.Name

	newInstance.PacketCluster.ObjectMeta.Name = instance.Name
	newInstance.PacketCluster.ObjectMeta.Namespace = namespace
	newInstance.PacketCluster.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.PacketCluster.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketCluster.Spec.ProjectID = common.GetPacketProjectID()
	newInstance.PacketCluster.Spec.Facility = instance.Facility

	newInstance.Cluster.ObjectMeta.Name = instance.Name
	newInstance.Cluster.ObjectMeta.Namespace = namespace
	newInstance.Cluster.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.Cluster.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	newInstance.Cluster.Spec.InfrastructureRef.Name = instance.Name
	newInstance.Cluster.Spec.ControlPlaneRef.Name = instance.Name + "-control-plane"

	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Namespace = namespace
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User

	newInstance.PacketMachineTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Namespace = namespace
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Labels["io.sharing.pair-instance"] = instance.Name
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Labels["io.sharing.pair-username"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplateWorker.Spec.Template.Spec.MachineType = "c1.small.x86"

	return err, newInstance
}

func KubernetesGetKubeconfigBytes(name string, clientset *kubernetes.Clientset) (err error, kubeconfigBytes []byte) {
	targetNamespace := common.GetTargetNamespace()
	secret, err := clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to get Kubernetes cluster Kubeconfig; err: %#v", err), []byte{}
	}
	kubeconfigBytes = secret.Data["value"]
	return err, kubeconfigBytes
}

func KubernetesGetKubeconfig(name string, clientset *kubernetes.Clientset) (err error, kubeconfig *clientcmdapi.Config) {
	err, valueBytes := KubernetesGetKubeconfigBytes(name, clientset)
	kubeconfig, err = clientcmd.Load(valueBytes)
	return err, kubeconfig
}

func KubernetesExec(clientset *kubernetes.Clientset, restConfig *rest.Config, options ExecOptions) (err error, stdout string, stderr string) {
	// TODO get restconfig from kubeconfig
	// TODO get rest client
	// TODO post exec
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(fmt.Sprintf("%s-humacs-0", options.PodName)).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       options.TTY,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return err, stdout, stderr
	}
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: &stdoutBuffer,
		Stderr: &stderrBuffer,
		Tty:    options.TTY,
	})
	if err != nil {
		return err, stdoutBuffer.String(), stderrBuffer.String()
	}

	if options.PreserveWhitespace {
		return err, stdoutBuffer.String(), stderrBuffer.String()
	}
	return err, strings.TrimSpace(stdoutBuffer.String()), strings.TrimSpace(stderrBuffer.String())

	// https://github.com/kubernetes/kubectl/blob/e65caf964573fbf671c4648032da4b7df7c7eaf0/pkg/cmd/exec/exec.go#L357
}

func KubernetesGetTmateSession(clientset *kubernetes.Clientset, name string) (err error, output string) {
	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(name, clientset)
	if err != nil {
		return err, output
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(instanceKubeconfig)
	if err != nil {
		return err, output
	}
	instanceClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err, output
	}

	execOptions := ExecOptions{
		Command: []string{
			"tmate",
			"-S",
			"/tmp/ii.default.target.iisocket",
			"display",
			"-p",
			"#{tmate_ssh}",
		},
		Namespace:          name,
		PodName:            name,
		ContainerName:      "humacs",
		CaptureStderr:      true,
		CaptureStdout:      true,
		PreserveWhitespace: false,
		TTY:                false,
	}
	err, stdout, stderr := KubernetesExec(instanceClientset, restConfig, execOptions)
	if stderr != "" {
		return fmt.Errorf(stderr), stdout
	}
	return err, stdout
}