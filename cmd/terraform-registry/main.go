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
	listenAddr     string
	authDisabled   bool
	authTokensFile string
	tlsEnabled     bool
	tlsCertFile    string
	tlsKeyFile     string
	storeType      string

	gitHubToken       string
	gitHubOwnerFilter string
	gitHubTopicFilter string
)

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":8080", "")
	flag.BoolVar(&authDisabled, "auth-disabled", false, "")
	flag.StringVar(&authTokensFile, "auth-tokens-file", "", "")
	flag.BoolVar(&tlsEnabled, "tls-enabled", false, "")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "", "")
	flag.StringVar(&storeType, "store", "", "store backend to use (choices: github)")

	gitHubToken = os.Getenv("GITHUB_TOKEN")
	flag.StringVar(&gitHubOwnerFilter, "github-owner-filter", "", "GitHub org/user repository filter")
	flag.StringVar(&gitHubTopicFilter, "github-topic-filter", "", "GitHub topic repository filter")
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

	// Configure the chosen store type
	switch storeType {
	case "github":
		gitHubRegistry(reg)
	default:
		log.Fatalln("error: invalid store type")
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

// gitHubRegistry configures the registry to use GitHubStore.
func gitHubRegistry(reg *registry.Registry) {
	if gitHubToken == "" {
		log.Fatalf("env var not set: GITHUB_TOKEN")
	}
	if gitHubOwnerFilter == "" {
		log.Fatalf("arg not set: -github-owner-filter")
	}
	if gitHubTopicFilter == "" {
		log.Fatalf("arg not set: -github-topic-filter")
	}

	store := github.NewGitHubStore(gitHubOwnerFilter, gitHubTopicFilter, gitHubToken)
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

// parseAuthTokensFile returns a slice of all non-empty strings found in the `filepath`.
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
