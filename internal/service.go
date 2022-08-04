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
	ThreadMatches prometheus.Gauge
}

type ServiceIngress struct {
	Namespace       string
	Name            string
	TopLevelDomains []string
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
	if err := s.UpdateThreadMatchMetrics(); err != nil {
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
			if err := s.UpdateIngresses(); err != nil {
				Error.Printf("Error: %v\n", err)
			}
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		for {
			if err := s.UpdateThreadMatchMetrics(); err != nil {
				Error.Printf("Error: %v\n", err)
			}
			time.Sleep(s.Opts.Interval)
		}
	}()

	select {
	case <-stop:
		return nil
	case err := <-errs:
		return err
	}
}

func (s *Service) PrometheusForTopLevelDomain(topLevelDomain string) *ServicePrometheus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prom, ok := s.Prometheus[topLevelDomain]; ok {
		return prom
	}

	promLabels := prometheus.Labels{
		"top_level_domain": topLevelDomain,
	}
	prom := &ServicePrometheus{
		ThreadMatches: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   prometheusNamespace,
			Name:        "thread_matches",
			ConstLabels: promLabels,
		}),
	}
	prometheus.MustRegister(prom.ThreadMatches)

	s.Prometheus[topLevelDomain] = prom
	return prom
}

func (s *Service) FindThreadMatches() (map[string]bool, error) {
	Debug.Printf("Retrieving google safebrowsing thread matches\n")

	topLevelDomains := []string{}
	for _, ingress := range s.Ingresses {
		topLevelDomains = append(topLevelDomains, ingress.TopLevelDomains...)
	}
	topLevelDomains = append(topLevelDomains, s.Opts.AdditionalDomains...)
	topLevelDomains = unique(topLevelDomains)

	ctx, cancel := context.WithTimeout(s.Context, 30*time.Second)
	defer cancel()
	sb, err := safebrowsing.NewSafeBrowser(safebrowsing.Config{
		APIKey: s.Opts.GoogleSafebrowsingApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieving google safebrowsing thread matches failed: %w", err)
	}
	threats, err := sb.LookupURLsContext(ctx, topLevelDomains)
	if err != nil {
		return nil, fmt.Errorf("retrieving google safebrowsing thread matches failed: %w", err)
	}

	result := map[string]bool{}
	for _, topLevelDomain := range topLevelDomains {
		result[topLevelDomain] = false
		for _, a := range threats {
			for _, b := range a {
				if b.Pattern == topLevelDomain+"/" {
					result[topLevelDomain] = true
				}
			}
		}
	}

	return result, nil
}

func (s *Service) UpdateThreadMatchMetrics() error {
	topLevelDomainThreads, err := s.FindThreadMatches()
	if err != nil {
		return nil
	}

	for topLevelDomain, threadsFound := range topLevelDomainThreads {
		prom := s.PrometheusForTopLevelDomain(topLevelDomain)
		if threadsFound {
			prom.ThreadMatches.Set(1.0)
		} else {
			prom.ThreadMatches.Set(0.0)
		}
	}

	for topLevelDomain, prom := range s.Prometheus {
		found := false
		for topLevelDomainThread, _ := range topLevelDomainThreads {
			if topLevelDomainThread == topLevelDomain {
				found = true
				break
			}
		}
		if !found {
			prom.ThreadMatches.Set(0.0)
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
			topLevelDomains := []string{}

			for _, rule := range ingress.Spec.Rules {
				hostSegments := strings.Split(rule.Host, ".")
				topLevelDomain := strings.Join(hostSegments[max(len(hostSegments)-2, 0):], ".")

				topLevelDomains = append(topLevelDomains, topLevelDomain)
			}
			topLevelDomains = unique(topLevelDomains)

			ingresses = append(ingresses, ServiceIngress{
				Namespace:       namespace,
				Name:            name,
				TopLevelDomains: topLevelDomains,
			})
			Debug.Printf("Found ingress %s/%s with top level domains %v\n", namespace, name, topLevelDomains)
		}
	}
	s.Ingresses = ingresses

	return nil
}
