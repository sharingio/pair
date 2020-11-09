package dns

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sharingio/pair/common"
)

type Entry struct {
	Subdomain string   `json:"subdomain"`
	Value     string `json:"values"`
}

var defaultContainerImage = "registry.gitlab.com/sharingio/pair/dns-update-job:latest"

func ScheduleDNSUpdateJob(clientset *kubernetes.Clientset, entry Entry) {
	targetNamespace := common.GetTargetNamespace()

	dnsUpdateJob := batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dns-update-"+entry.Subdomain,
			Labels: map[string]string{
				"io.sharing.pair": "job",
			},
		},
		Spec: batch.JobSpec{
			Selector: &metav1.LabelSelector{
				"io.sharing.pair-subdomain": entry.Subdomain,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"io.sharing.pair": "job",
						"io.sharing.pair-job-name": entry.Subdomain,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "dns-update",
						Image: defaultContainerImage,
						Env: []corev1.EnvVar{
							{
								Name: "DOMAIN",
								Value: common.GetInstanceSubdomain(),
							},
							{
								Name: "SUBDOMAIN",
								Value: entry.Subdomain,
							},
							{
								Name: "ADDRESS",
								Value: entry.Value,
							},
							{
								Name: "AWS_ACCESS_KEY_ID",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Name: common.GetKubernetesSecretName(),
										Key: "awsAccessKeyID",
									},
								},
							},
							{
								Name: "AWS_SECRET_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Name: common.GetKubernetesSecretName(),
										Key: "awsSecretAccessKey",
									},
								},
							},
						},
					}},
				},
			},
		},
	}
	clientset.BatchV1().Job(targetNamespace).Create(context.TODO(), &dnsUpdateJob, metav1.CreateOptions{})
}
