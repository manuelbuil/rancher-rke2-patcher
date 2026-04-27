package kube

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type HelmChartObject struct {
	Name      string
	Namespace string
	Content   string
}

// ListHelmChartsByIdentity lists HelmChart objects in the cluster that match the given name and namespace.
func ListHelmChartsByIdentity(name string, namespace string) ([]HelmChartObject, error) {
	trimmedName := strings.TrimSpace(name)
	trimmedNamespace := strings.TrimSpace(namespace)
	if trimmedName == "" {
		return nil, fmt.Errorf("helmchart name cannot be empty")
	}
	if trimmedNamespace == "" {
		return nil, fmt.Errorf("helmchart namespace cannot be empty")
	}

	dynamicClient, err := kubeDynamicClient()
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "helm.cattle.io", Version: "v1", Resource: "helmcharts"}
	list, err := dynamicClient.Resource(gvr).Namespace(trimmedNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list helmcharts in %s: %w", trimmedNamespace, err)
	}

	results := make([]HelmChartObject, 0, len(list.Items))
	for _, item := range list.Items {
		if strings.TrimSpace(item.GetName()) != trimmedName {
			continue
		}
		if strings.TrimSpace(item.GetNamespace()) != trimmedNamespace {
			continue
		}

		content, err := item.MarshalJSON()
		if err != nil {
			return nil, err
		}

		results = append(results, HelmChartObject{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Content:   string(content),
		})
	}

	return results, nil
}
