// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/nrkno/terraform-registry/pkg/registry"
	"github.com/nrkno/terraform-registry/pkg/store/github"
)

var (
	WelcomeMessage = []byte("Terraform Registry\nhttps://github.com/nrkno/terraform-registry\n")
)

func main() {
	log.Default().SetFlags(log.Lshortfile)

	reg := new(registry.Registry)
	envconfig.MustProcess("", reg)
	reg.SetupRoutes()

	if !reg.IsAuthDisabled {
		if err := reg.LoadAuthTokens(); err != nil {
			log.Fatalf("error: failed to load auth tokens: %v", err)
		}
		log.Println("info: authentication is enabled")
		log.Printf("info: loaded %d auth tokens", len(reg.GetAuthTokens()))
	} else {
		log.Println("warning: authentication is disabled")
	}

	store := github.NewGitHubStore(reg.GitHubOrgName, reg.GitHubRepoTopic, reg.GitHubToken)
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

	srv := http.Server{
		Addr:              reg.ListenAddr,
		Handler:           reg,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second, // keep-alive timeout
	}

	if reg.TLSEnabled {
		log.Printf("info: Starting HTTP server (TLS enabled), listening on %s", reg.ListenAddr)
		log.Fatalf("error: %v", srv.ListenAndServeTLS(reg.TLSCertFile, reg.TLSKeyFile))
	} else {
		log.Printf("info: Starting HTTP server (TLS disabled), listening on %s", reg.ListenAddr)
		log.Fatalf("error: %v", srv.ListenAndServe())
	}
}
