// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nrkno/terraform-registry/pkg/registry"
	"github.com/nrkno/terraform-registry/pkg/store/github"
)

var (
	registryType string

	listenAddr     string
	authDisabled   bool
	authTokensFile string
	tlsEnabled     bool
	tlsCertFile    string
	tlsKeyFile     string

	gitHubToken     string
	gitHubOrgName   string
	gitHubRepoTopic string
)

func init() {
	flag.StringVar(&registryType, "store", "", "module store implementation to use (choices: github)")

	flag.StringVar(&listenAddr, "listen-addr", ":8080", "")
	flag.BoolVar(&authDisabled, "auth-disabled", false, "")
	flag.StringVar(&authTokensFile, "auth-tokens-file", "", "")
	flag.BoolVar(&tlsEnabled, "tls-enabled", false, "")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "", "")

	gitHubToken = os.Getenv("GITHUB_TOKEN")
	flag.StringVar(&gitHubOrgName, "github-org", "", "GitHub org to find repositories in")
	flag.StringVar(&gitHubRepoTopic, "github-topic", "", "GitHub topic to find repositories in")
}

func main() {
	flag.Parse()
	log.Default().SetFlags(log.Lshortfile)

	reg := registry.NewRegistry()
	reg.IsAuthDisabled = authDisabled
	reg.AuthTokenFile = authTokensFile

	if !reg.IsAuthDisabled {
		tokens, err := parseAuthTokensFile(authTokensFile)
		if err != nil {
			log.Fatalf("error: failed to load auth tokens: %v", err)
		}
		if len(tokens) == 0 {
			log.Fatalf("error: no tokens found in auth token file")
		}
		reg.SetAuthTokens(tokens)

		log.Println("info: HTTP authentication enabled")
		log.Printf("info: loaded %d auth tokens", len(tokens))
	} else {
		log.Println("warning: HTTP authentication disabled")
	}

	// Configure the chosen registry type
	switch registryType {
	case "github":
		gitHubRegistry(reg)
	default:
		log.Fatalln("error: invalid registry type")
	}

	srv := http.Server{
		Addr:              listenAddr,
		Handler:           reg,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second, // keep-alive timeout
	}

	if tlsEnabled {
		log.Printf("info: Starting HTTP server (TLS enabled), listening on %s", listenAddr)
		log.Fatalf("error: %v", srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile))
	} else {
		log.Printf("info: Starting HTTP server (TLS disabled), listening on %s", listenAddr)
		log.Fatalf("error: %v", srv.ListenAndServe())
	}
}

// Configure the registry to use GitHub as a module backend.
func gitHubRegistry(reg *registry.Registry) {
	if gitHubToken == "" {
		log.Fatalf("env var not set: GITHUB_TOKEN")
	}
	if gitHubOrgName == "" {
		log.Fatalf("arg not set: -github-org")
	}
	if gitHubRepoTopic == "" {
		log.Fatalf("arg not set: -github-topic")
	}

	store := github.NewGitHubStore(gitHubOrgName, gitHubRepoTopic, gitHubToken)
	reg.SetModuleStore(store)

	// Fill store cache initially
	log.Println("info: loading GitHub store cache")
	if err := store.ReloadCache(context.Background()); err != nil {
		log.Fatalf("error: failed to load store cache: %v", err)
	}

	// Reload store cache on regular intervals
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		<-t.C // ignore the first tick

		for {
			log.Println("info: reloading store cache")
			if err := store.ReloadCache(context.Background()); err != nil {
				log.Printf("error: failed to load store cache: %v", err)
			}
			<-t.C
		}
	}()
}

// LoadAuthTokens loads valid auth tokens from the configured `app.AuthTokenFile`.
func parseAuthTokensFile(filepath string) ([]string, error) {
	var tokens []string

	b, err := os.ReadFile(filepath)
	if err != nil {
		return tokens, err
	}

	lines := strings.Split(string(b), "\n")
	for _, token := range lines {
		if token = strings.TrimSpace(token); token != "" {
			tokens = append(tokens, token)
		}
	}

	return tokens, nil
}
