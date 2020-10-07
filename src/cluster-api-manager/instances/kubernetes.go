package instances

import (
	"context"
	"fmt"
	"log"

	"github.com/sharingio/pair/src/cluster-api-manager/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	// "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/kubectl/pkg/scheme"
	clusterAPIPacketv1alpha3 "sigs.k8s.io/cluster-api-provider-packet/api/v1alpha3"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	// "sigs.k8s.io/yaml"
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

func Int32ToInt32Pointer(input int32) *int32 {
	return &input
}

var defaultMachineOS = "ubuntu_20_04"
var defaultKubernetesVersion = "1.19.0"
var defaultKubernetesClusterConfig = KubernetesCluster{
	KubeadmControlPlane: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Spec: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneSpec{
			Version:  "1.19.0",
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
					// 1 = packet project ID
					// 2 = cluster name
					// TODO remove
					"kubectl create secret generic -n kube-system packet-cloud-config --from-literal=cloud-sa.json='{\"apiKey\": \"{{ .apiKey }}\",\"projectID\": \"%s\", \"eipTag\": \"cluster-api-provider-packet:cluster-id:%s\"}'",
					"kubectl taint node --all node-role.kubernetes.io/master-",
					// TODO remove
					"kubectl apply -f https://github.com/packethost/packet-ccm/releases/download/v1.1.0/deployment.yaml",
					// TODO remove
					"kubectl apply -f https://github.com/packethost/csi-packet/raw/master/deploy/kubernetes/setup.yaml",
					// TODO remove
					"kubectl apply -f https://github.com/packethost/csi-packet/raw/master/deploy/kubernetes/node.yaml",
					// TODO remove
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
      EOF
`,
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
          helm install "%s" -n "%s" -f chart/humacs/values/apisnoop.yaml --set options.timezone="%s" chart/humacs
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
			Name: "",
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
			},
		},
	},
	MachineDeploymentWorker: clusterAPIv1alpha3.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
			Labels: map[string]string{
				"pool": "worker-a",
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
						"pool": "worker-a",
					},
				},
				Spec: clusterAPIv1alpha3.MachineSpec{
					Version: &defaultKubernetesVersion,
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
			Name: "",
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
			Name: "",
		},
		Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
			Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
				Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
					OS:           defaultMachineOS,
					BillingCycle: "hourly",
					// 1 = machine type
					MachineType: "%s",
				},
			},
		},
	},
	PacketCluster: clusterAPIPacketv1alpha3.PacketCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Spec: clusterAPIPacketv1alpha3.PacketClusterSpec{},
	},
	PacketMachineTemplateWorker: clusterAPIPacketv1alpha3.PacketMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
			Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
				Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
					OS:           defaultMachineOS,
					BillingCycle: "hourly",
					// 1 = machine type
					MachineType: "%s",
				},
			},
		},
	},
}

func KubernetesGet(name string) (err error, instance InstanceSpec) {
	return err, instance
}

func KubernetesList() (err error, instances []InstanceSpec) {
	return err, instances
}

func KubernetesCreate(instance InstanceSpec, kubernetesClientset dynamic.Interface) (err error, instanceCreated InstanceSpec) {
	// generate name
	instance.Name = "something" // + random string 6 chars
	targetNamespace := "default"
	var newInstance = defaultKubernetesClusterConfig

	newInstance.KubeadmControlPlane.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.ObjectMeta.Namespace = targetNamespace
	newInstance.KubeadmControlPlane.Spec.InfrastructureTemplate.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[5] = fmt.Sprintf(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[5], "" /*projectID*/, instance.Name)
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[19] = fmt.Sprintf(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[19], instance.Name, instance.Name, instance.Name, instance.Setup.Timezone)

	newInstance.PacketMachineTemplate.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.PacketMachineTemplate.ObjectMeta.Namespace = targetNamespace
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplate.Spec.Template.Spec.MachineType = "c1.small.x86"

	newInstance.MachineDeploymentWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.ObjectMeta.Namespace = targetNamespace
	newInstance.MachineDeploymentWorker.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.ClusterName = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.Bootstrap.ConfigRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Selector.MatchLabels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.InfrastructureRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.ClusterName = instance.Name

	newInstance.PacketCluster.ObjectMeta.Name = instance.Name
	newInstance.PacketCluster.ObjectMeta.Namespace = targetNamespace
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketCluster.Spec.ProjectID = "something"
	newInstance.PacketCluster.Spec.Facility = instance.Facility

	newInstance.Cluster.ObjectMeta.Name = instance.Name
	newInstance.Cluster.ObjectMeta.Namespace = targetNamespace
	newInstance.Cluster.Spec.InfrastructureRef.Name = instance.Name
	newInstance.Cluster.Spec.ControlPlaneRef.Name = instance.Name + "-control-plane"

	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Namespace = targetNamespace

	newInstance.PacketMachineTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Namespace = targetNamespace
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplateWorker.Spec.Template.Spec.MachineType = "c1.small.x86"

	// manifests
	// TODO create all the things
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

func KubernetesDelete(instance InstanceSpec) (err error) {
	return err
}
