package ocm

import (
	"context"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func getValues(restConfig *rest.Config) addonfactory.GetValuesFunc {
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
			if found {
				// Map to Helm chart values format
				values := map[string]interface{}{
					"controllerSecret": map[string]interface{}{
						"namespace": ref.Namespace,
					},
					"cluster": map[string]interface{}{
						"name": ref.Name,
					},
					"netop": map[string]interface{}{
						"networkInterface": specMap["networkInterface"],
					},
				}
				overrideValues = addonfactory.MergeValues(overrideValues, values)
			}
		}

		return overrideValues, nil
	}
}
