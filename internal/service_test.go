package internal

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd"
)

func TestServiceUpdateIngresses(t *testing.T) {
	service, err := createService()
	if err != nil {
		t.Fatal(err)
	}

	if err := service.UpdateIngresses(); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []ServiceIngress{
		{Namespace: "default", Name: "ingress-1", TopLevelDomains: []string{"ingress-1.localhost"}},
		{Namespace: "default", Name: "ingress-2", TopLevelDomains: []string{"ingress-2.localhost"}},
		{Namespace: "default", Name: "ingress-3", TopLevelDomains: []string{"ingress-3a.localhost", "ingress-3b.localhost"}},
	}, service.Ingresses)
}

func TestServiceFindThreadMatches(t *testing.T) {
	service, err := createService()
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.FindThreadMatches()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, map[string]bool{"microsoftofficeonedrivefileshare.on.fleek.co": true}, result)
}

func createService() (*Service, error) {
	googleSafebrowsingApiKey := os.Getenv("GOOGLE_SAFEBROWSING_API_KEY")
	if googleSafebrowsingApiKey == "" {
		return nil, fmt.Errorf("environment variable GOOGLE_SAFEBROWSING_API_KEY is missing")
	}

	apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, nil)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	service, err := NewService(ServiceOpts{
		Interval:                 5 * time.Second,
		KubernetesRestConfig:     config,
		GoogleSafebrowsingApiKey: googleSafebrowsingApiKey,
		AdditionalDomains:        []string{"microsoftofficeonedrivefileshare.on.fleek.co"},
	})
	if err != nil {
		return nil, err
	}
	return service, nil
}
