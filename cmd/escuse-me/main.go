package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "escuse-me",
	Short: "GO GO GOLEM ESCUSE ME ELASTIC SEARCH GADGET",
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringSlice("addresses", []string{"http://localhost:9200"}, "Elasticsearch addresses")
	rootCmd.PersistentFlags().String("username", "", "Elasticsearch username")
	rootCmd.PersistentFlags().String("password", "", "Elasticsearch password")
	rootCmd.PersistentFlags().String("cloud-id", "", "Elasticsearch cloud ID")
	rootCmd.PersistentFlags().String("api-key", "", "Elasticsearch API key")
	rootCmd.PersistentFlags().String("service-token", "", "Elasticsearch service token")
	rootCmd.PersistentFlags().String("certificate-fingerprint", "", "Elasticsearch certificate fingerprint")
	rootCmd.PersistentFlags().IntSlice("retry-on-status", []int{502, 503, 504, 429}, "Elasticsearch retry on status")
	rootCmd.PersistentFlags().Bool("disable-retry", false, "Elasticsearch disable retry")
	rootCmd.PersistentFlags().Int("max-retries", 3, "Elasticsearch max retries")
	rootCmd.PersistentFlags().Bool("enable-metrics", false, "Elasticsearch enable metrics")
	rootCmd.PersistentFlags().Bool("enable-debug-logger", false, "Elasticsearch enable debug logger")
	rootCmd.PersistentFlags().Bool("enable-compatibility-mode", false, "Elasticsearch enable compatibility mode")
	err := InitViper("escuse-me", "")
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(infoCmd)
}
