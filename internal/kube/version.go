package kube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type clusterVersionResponse struct {
	GitVersion string `json:"gitVersion"`
}

func ClusterVersion() (string, error) {
	api, err := kubeAPIClient()
	if err != nil {
		return "", err
	}

	return clusterVersion(api)
}

func clusterVersion(api kubeAPI) (string, error) {
	requestURL := api.BaseURL + "/version"

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(api.AuthHeader) != "" {
		req.Header.Set("Authorization", api.AuthHeader)
	}

	resp, err := api.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("kube api returned status %d while fetching cluster version: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var response clusterVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	version := strings.TrimSpace(response.GitVersion)
	if version == "" {
		return "", fmt.Errorf("kube api response did not include gitVersion")
	}

	return version, nil
}
