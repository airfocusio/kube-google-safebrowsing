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
		{Namespace: "default", Name: "ingress-1", Domains: []string{"ingress-1.localhost"}},
		{Namespace: "default", Name: "ingress-2", Domains: []string{"ingress-2.localhost"}},
		{Namespace: "default", Name: "ingress-3", Domains: []string{"ingress-3a.localhost", "ingress-3b.localhost"}},
		{Namespace: "default", Name: "ingress-4", Domains: []string{"sub.ingress-4.localhost"}},
	}, service.Ingresses)
}

func TestServiceFindThreatMatches(t *testing.T) {
	service, err := createService()
	if err != nil {
		t.Fatal(err)
	}

	if err := service.UpdateIngresses(); err != nil {
		t.Fatal(err)
	}
	result, err := service.FindThreatMatches()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, map[string]bool{
		"ingress-1.localhost":                          false,
		"ingress-2.localhost":                          false,
		"ingress-3a.localhost":                         false,
		"ingress-3b.localhost":                         false,
		"sub.ingress-4.localhost":                      false,
		"ingress-4.localhost":                          false,
		"microsoftofficeonedrivefileshare.on.fleek.co": true,
	}, result)
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
