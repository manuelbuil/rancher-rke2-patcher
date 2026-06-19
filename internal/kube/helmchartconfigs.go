package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"gopkg.in/yaml.v3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type HelmChartConfigObject struct {
	Name      string
	Namespace string
	Content   string
}

type helmChartConfigItem struct {
	APIVersion string         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string         `json:"kind" yaml:"kind"`
	Metadata   helmObjectMeta `json:"metadata" yaml:"metadata"`
	Spec       map[string]any `json:"spec" yaml:"spec"`
}

type helmObjectMeta struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

// kubeDynamicClient returns a dynamic.Interface using in-cluster config if available, otherwise falls back to kubeconfig.
func kubeDynamicClient() (dynamic.Interface, error) {
	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig
		kubeconfigPath := os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			kubeconfigPath = "/etc/rancher/rke2/rke2.yaml"
		}
		if _, statErr := os.Stat(kubeconfigPath); statErr != nil {
			home, homeErr := os.UserHomeDir()
			if homeErr == nil {
				kubeconfigPath = home + "/.kube/config"
			}
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes dynamic client: %w", err)
	}
	return dynamicClient, nil
}

// GetHelmChartConfigByIdentity is a variable for testability, delegates to getHelmChartConfigByIdentityImpl.
var GetHelmChartConfigByIdentity = getHelmChartConfigByIdentityImpl

func getHelmChartConfigByIdentityImpl(name string, namespace string) (*HelmChartConfigObject, error) {
	trimmedName := strings.TrimSpace(name)
	trimmedNamespace := strings.TrimSpace(namespace)
	if trimmedName == "" {
		return nil, fmt.Errorf("helmchartconfig name cannot be empty")
	}
	if trimmedNamespace == "" {
		return nil, fmt.Errorf("helmchartconfig namespace cannot be empty")
	}

	dynamicClient, err := kubeDynamicClient()
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "helm.cattle.io", Version: "v1", Resource: "helmchartconfigs"}
	item, err := dynamicClient.Resource(gvr).Namespace(trimmedNamespace).Get(context.Background(), trimmedName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get helmchartconfig %s/%s: %w", trimmedNamespace, trimmedName, err)
	}

	spec, _, err := unstructured.NestedMap(item.Object, "spec")
	if err != nil {
		return nil, err
	}

	manifest := helmChartConfigItem{
		APIVersion: item.GetAPIVersion(),
		Kind:       item.GetKind(),
		Metadata: helmObjectMeta{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		},
		Spec: spec,
	}
	if strings.TrimSpace(manifest.APIVersion) == "" {
		manifest.APIVersion = "helm.cattle.io/v1"
	}
	if strings.TrimSpace(manifest.Kind) == "" {
		manifest.Kind = "HelmChartConfig"
	}

	contentBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return &HelmChartConfigObject{
		Name:      item.GetName(),
		Namespace: item.GetNamespace(),
		Content:   string(contentBytes),
	}, nil
}

// ApplyHelmChartConfig is a variable for testability, delegates to applyHelmChartConfigImpl.
var ApplyHelmChartConfig = applyHelmChartConfigImpl

// applyHelmChartConfigImpl applies a HelmChartConfig via server-side apply.
func applyHelmChartConfigImpl(chart *helmv1.HelmChartConfig) error {
	if chart == nil {
		return fmt.Errorf("helmchartconfig cannot be nil")
	}

	name := strings.TrimSpace(chart.GetName())
	if name == "" {
		return fmt.Errorf("helmchartconfig name cannot be empty")
	}

	namespace := strings.TrimSpace(chart.GetNamespace())
	if namespace == "" {
		namespace = "kube-system"
		chart.SetNamespace(namespace)
	}

	gvr := schema.GroupVersionResource{Group: "helm.cattle.io", Version: "v1", Resource: "helmchartconfigs"}

	dynamicClient, err := kubeDynamicClient()
	if err != nil {
		return err
	}

	resource := dynamicClient.Resource(gvr).Namespace(namespace)

	patchBytes, err := json.Marshal(chart)
	if err != nil {
		return fmt.Errorf("failed to marshal HelmChartConfig for apply patch: %w", err)
	}

	force := true
	_, err = resource.Patch(
		context.Background(),
		name,
		types.ApplyPatchType,
		patchBytes,
		metav1.PatchOptions{
			FieldManager: "rke2-patcher",
			Force:        &force,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to apply helmchartconfig %s/%s: %w", namespace, name, err)
	}

	return nil
}
