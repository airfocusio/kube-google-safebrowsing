package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/safebrowsing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ServiceOpts struct {
	KubernetesRestConfig     *rest.Config
	GoogleSafebrowsingApiKey string
	Interval                 time.Duration
	AdditionalDomains        []string
}

type Service struct {
	mu                  sync.Mutex
	Opts                ServiceOpts
	Context             context.Context
	KubernetesClientset *kubernetes.Clientset
	Ingresses           []ServiceIngress
	Prometheus          map[string]*ServicePrometheus
}

type ServicePrometheus struct {
	ThreatMatches prometheus.Gauge
}

type ServiceIngress struct {
	Namespace string
	Name      string
	Domains   []string
}

const prometheusNamespace = "google_safebrowsing"

func NewService(opts ServiceOpts) (*Service, error) {
	service := &Service{Opts: opts}
	service.Context = context.Background()

	clientset, err := kubernetes.NewForConfig(opts.KubernetesRestConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create kubernetes rest clientset: %w", err)
	}
	service.KubernetesClientset = clientset
	service.Ingresses = []ServiceIngress{}
	service.Prometheus = map[string]*ServicePrometheus{}

	return service, nil
}

func (s *Service) Run(stop <-chan os.Signal) error {
	errs := make(chan error)

	Info.Printf("Initializing\n")
	if err := s.UpdateIngresses(); err != nil {
		return err
	}
	if err := s.UpdateThreatMatchMetrics(); err != nil {
		return err
	}

	go func() {
		sm := http.NewServeMux()
		sm.Handle("/metrics", promhttp.Handler())

		Info.Printf("Starting metrics server\n")
		if err := http.ListenAndServe(fmt.Sprintf(":%d", 1024), sm); err != nil {
			errs <- fmt.Errorf("unable to start http stat server: %w", err)
			return
		}

		errs <- nil
	}()

	go func() {
		for {
			time.Sleep(s.Opts.Interval)
			if err := s.UpdateIngresses(); err != nil {
				Error.Printf("Error: %v\n", err)
			}
			if err := s.UpdateThreatMatchMetrics(); err != nil {
				Error.Printf("Error: %v\n", err)
			}
		}
	}()

	select {
	case <-stop:
		return nil
	case err := <-errs:
		return err
	}
}

func (s *Service) PrometheusForDomain(domain string) *ServicePrometheus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prom, ok := s.Prometheus[domain]; ok {
		return prom
	}

	promLabels := prometheus.Labels{
		"domain": domain,
	}
	prom := &ServicePrometheus{
		ThreatMatches: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   prometheusNamespace,
			Name:        "threat_matches",
			ConstLabels: promLabels,
		}),
	}
	prometheus.MustRegister(prom.ThreatMatches)

	s.Prometheus[domain] = prom
	return prom
}

func (s *Service) FindThreatMatches() (map[string]bool, error) {
	Debug.Printf("Retrieving google safebrowsing threat matches\n")

	domains := []string{}
	for _, ingress := range s.Ingresses {
		domains = append(domains, ingress.Domains...)
	}
	domains = append(domains, s.Opts.AdditionalDomains...)
	domains = unique(domains)

	ctx, cancel := context.WithTimeout(s.Context, 30*time.Second)
	defer cancel()
	sb, err := safebrowsing.NewSafeBrowser(safebrowsing.Config{
		APIKey: s.Opts.GoogleSafebrowsingApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieving google safebrowsing threat matches failed: %w", err)
	}
	threats, err := sb.LookupURLsContext(ctx, domains)
	if err != nil {
		return nil, fmt.Errorf("retrieving google safebrowsing threat matches failed: %w", err)
	}

	result := map[string]bool{}
	for _, domain := range domains {
		result[domain] = false
		for _, a := range threats {
			for _, b := range a {
				if b.Pattern == domain+"/" {
					result[domain] = true
				}
			}
		}
	}

	return result, nil
}

func (s *Service) UpdateThreatMatchMetrics() error {
	domainThreats, err := s.FindThreatMatches()
	if err != nil {
		return nil
	}

	for domain, threatsFound := range domainThreats {
		prom := s.PrometheusForDomain(domain)
		if threatsFound {
			prom.ThreatMatches.Set(1.0)
		} else {
			prom.ThreatMatches.Set(0.0)
		}
	}

	for domain, prom := range s.Prometheus {
		found := false
		for domainThreat, _ := range domainThreats {
			if domainThreat == domain {
				found = true
				break
			}
		}
		if !found {
			prom.ThreatMatches.Set(0.0)
		}
	}

	return nil
}

func (s *Service) UpdateIngresses() error {
	Debug.Printf("Updating ingresses\n")

	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(s.Context, 30*time.Second)
	defer cancel()

	namespaceList, err := s.KubernetesClientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("updating ingresses failed: %w", err)
	}

	ingresses := []ServiceIngress{}
	for _, namespace := range namespaceList.Items {
		ingressList, err := s.KubernetesClientset.NetworkingV1().Ingresses(namespace.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("updating ingresses failed: %w", err)
		}

		for _, ingress := range ingressList.Items {
			namespace := ingress.ObjectMeta.Namespace
			name := ingress.ObjectMeta.Name
			domains := []string{}

			for _, rule := range ingress.Spec.Rules {
				domains = append(domains, strings.TrimPrefix(rule.Host, "*."))
			}
			domains = unique(domains)

			ingresses = append(ingresses, ServiceIngress{
				Namespace: namespace,
				Name:      name,
				Domains:   domains,
			})
			Debug.Printf("Found ingress %s/%s with domains %v\n", namespace, name, domains)
		}
	}
	s.Ingresses = ingresses

	return nil
}
