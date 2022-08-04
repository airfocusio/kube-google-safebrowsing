package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airfocusio/kube-google-safebrowsing/internal"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	verbose                  bool
	rootCmdKubeConfig        bool
	rootCmdInterval          time.Duration
	rootCmdAdditionalDomains []string
	rootCmd                  = &cobra.Command{
		Use: "kube-google-safebrowsing",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadKubernetsRestConfig(rootCmdKubeConfig)
			if err != nil {
				return fmt.Errorf("unable to create kubernetes rest config: %w", err)
			}
			googleSafebrowsingApiKey := os.Getenv("GOOGLE_SAFEBROWSING_API_KEY")
			if googleSafebrowsingApiKey == "" {
				return fmt.Errorf("environment variable GOOGLE_SAFEBROWSING_API_KEY is missing")
			}
			service, err := internal.NewService(internal.ServiceOpts{
				KubernetesRestConfig:     config,
				GoogleSafebrowsingApiKey: googleSafebrowsingApiKey,
				Interval:                 rootCmdInterval,
				AdditionalDomains:        rootCmdAdditionalDomains,
			})
			if err != nil {
				return err
			}

			term := make(chan os.Signal, 1)
			signal.Notify(term, syscall.SIGTERM)
			signal.Notify(term, syscall.SIGINT)
			return service.Run(term)
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if !verbose {
				internal.Debug = log.New(ioutil.Discard, "", log.LstdFlags)
			}
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func loadKubernetsRestConfig(kubeConfig bool) (*rest.Config, error) {
	if kubeConfig {
		apiConfig, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			return nil, err
		}
		clientConfig := clientcmd.NewDefaultClientConfig(*apiConfig, nil)
		return clientConfig.ClientConfig()
	}

	return rest.InClusterConfig()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "")
	rootCmd.Flags().BoolVar(&rootCmdKubeConfig, "kube-config", false, "")
	rootCmd.Flags().DurationVar(&rootCmdInterval, "interval", 5*time.Minute, "")
	rootCmd.Flags().StringArrayVar(&rootCmdAdditionalDomains, "additional-domains", []string{}, "")
	rootCmd.AddCommand(versionCmd)
}
