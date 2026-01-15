package ocm

import (
	"context"
	"encoding/base64"
	"fmt"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"gomodules.xyz/cert/certstore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

const (
	secretPrefix = "kubeslice-rbac-worker-"
)

func getEncodedValue(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func getKubesliceSecert(kubeConfig *rest.Config, ref v1alpha1.ConfigReference) (*corev1.Secret, error) {
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	secretName := secretPrefix + ref.Name
	secretNS := ref.Namespace
	secret, err := clientset.CoreV1().Secrets(secretNS).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", secretNS, secretName, err)
	}
	return secret, nil
}

func getValues(restConfig *rest.Config, cs *certstore.CertStore) addonfactory.GetValuesFunc {
	return func(cluster *clusterv1.ManagedCluster, addon *v1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {
		overrideValues := make(addonfactory.Values)

		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}

		gvr := schema.GroupVersionResource{
			Group:    "controller.kubeslice.io",
			Version:  "v1alpha1",
			Resource: "clusters",
		}
		// Loop over ConfigReferences in ManagedClusterAddOn
		for _, ref := range addon.Status.ConfigReferences {
			if ref.ConfigGroupResource.Group != controllerv1alpha1.GroupVersion.Group ||
				ref.ConfigGroupResource.Resource != controllerv1alpha1.ResourceClusterConfigs {
				continue
			}

			// Prepare unstructured object
			clusterCR := &unstructured.Unstructured{}
			clusterCR.SetAPIVersion("controller.kubeslice.io/v1alpha1")
			clusterCR.SetKind("Cluster")
			clusterCR.SetName(ref.Name)
			clusterCR.SetNamespace(ref.Namespace)

			// Check if it exists
			_, err := dynamicClient.Resource(gvr).Namespace(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Create CR if it doesn't exist
					spec := map[string]interface{}{
						"networkInterface": "enp1s0",
						"clusterProperty":  map[string]interface{}{},
					}
					if err := unstructured.SetNestedMap(clusterCR.Object, spec, "spec"); err != nil {
						return nil, err
					}
					_, err := dynamicClient.Resource(gvr).Namespace(ref.Namespace).Create(context.TODO(), clusterCR, metav1.CreateOptions{})
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}

			// Extract spec for Helm values
			fetchedCR, err := dynamicClient.Resource(gvr).Namespace(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			specMap, found, err := unstructured.NestedMap(fetchedCR.Object, "spec")
			if err != nil {
				return nil, err
			}

			kubesliceSecret, err := getKubesliceSecert(restConfig, ref)
			if err != nil {
				return nil, err
			}

			caCrtBytes, _, err := cs.ReadBytes(CACertName)
			if err != nil {
				return nil, err
			}

			crtBytes, keyBytes, err := cs.ReadBytes(ServerCertName)
			if err != nil {
				return nil, err
			}

			if found {
				// Map to Helm chart values format
				values := map[string]interface{}{
					"controllerSecret": map[string]interface{}{
						"namespace": getEncodedValue(ref.Namespace),
						"endpoint":  getEncodedValue(string(kubesliceSecret.Data["controllerEndpoint"])),
						"ca.crt":    getEncodedValue(string(kubesliceSecret.Data["ca.crt"])),
						"token":     getEncodedValue(string(kubesliceSecret.Data["token"])),
					},
					"cluster": map[string]interface{}{
						"name": ref.Name,
					},
					"netop": map[string]interface{}{
						"networkInterface": specMap["networkInterface"],
					},
					"nsm": map[string]interface{}{
						"admission-webhook": map[string]interface{}{
							"apiserver": map[string]interface{}{
								"servingCerts": map[string]interface{}{
									"generate":  false,
									"caCrt":     getEncodedValue(string(caCrtBytes)),
									"serverCrt": getEncodedValue(string(crtBytes)),
									"serverKey": getEncodedValue(string(keyBytes)),
								},
							},
						},
					},
					"apiserver": map[string]interface{}{
						"servingCerts": map[string]interface{}{
							"generate":  false,
							"caCrt":     getEncodedValue(string(caCrtBytes)),
							"serverCrt": getEncodedValue(string(crtBytes)),
							"serverKey": getEncodedValue(string(keyBytes)),
						},
					},
					"ocm": map[string]interface{}{
						"enabled": true,
					},
				}
				overrideValues = addonfactory.MergeValues(overrideValues, values)
			}
		}

		return overrideValues, nil
	}
}
