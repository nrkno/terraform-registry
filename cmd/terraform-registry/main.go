// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nrkno/terraform-registry/pkg/registry"
	"github.com/nrkno/terraform-registry/pkg/store/github"
)

var (
	listenAddr     string
	authDisabled   bool
	authTokensFile string
	envJSONFiles   string
	tlsEnabled     bool
	tlsCertFile    string
	tlsKeyFile     string
	storeType      string

	gitHubToken       string
	gitHubOwnerFilter string
	gitHubTopicFilter string

	// > Environment variable names used by the utilities in the Shell and Utilities
	// > volume of IEEE Std 1003.1-2001 consist solely of uppercase letters, digits,
	// > and the '_' (underscore) from the characters defined in Portable Character
	// > Set and do not begin with a digit. Other characters may be permitted by an
	// > implementation; applications shall tolerate the presence of such names.
	// https://pubs.opengroup.org/onlinepubs/000095399/basedefs/xbd_chap08.html
	patternEnvVarName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*`)
)

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":8080", "")
	flag.BoolVar(&authDisabled, "auth-disabled", false, "")
	flag.StringVar(&authTokensFile, "auth-tokens-file", "", "")
	flag.StringVar(&envJSONFiles, "env-json-files", "", "List of comma-separated paths to JSON files. Converts the keys to uppercase and replaces all '-' with '_'. Prefix filepaths with 'myprefix:' to use a prefix names of the variables from a specific file.")
	flag.BoolVar(&tlsEnabled, "tls-enabled", false, "")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "", "")
	flag.StringVar(&storeType, "store", "", "store backend to use (choices: github)")

	flag.StringVar(&gitHubOwnerFilter, "github-owner-filter", "", "GitHub org/user repository filter")
	flag.StringVar(&gitHubTopicFilter, "github-topic-filter", "", "GitHub topic repository filter")
}

func main() {
	flag.Parse()
	log.Default().SetFlags(log.Lshortfile)

	if len(os.Args[1:]) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load environment from files
	for _, item := range strings.Split(envJSONFiles, ",") {
		prefix := ""
		filename := item
		// If the filename is prefixed, the prefix must be separated from the filename.
		if split := strings.SplitN(filename, ":", 2); len(split) == 2 {
			prefix = split[0]
			filename = split[1]
		}
		if err := setEnvironmentFromJSONFile(prefix, filename); err != nil {
			log.Fatalf("failed to load environment from file(s): %v", err)
		}
	}

	// Load environment variables here!
	gitHubToken = os.Getenv("GITHUB_TOKEN")

	reg := registry.NewRegistry()
	reg.IsAuthDisabled = authDisabled

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
	if gitHubOwnerFilter == "" && gitHubTopicFilter == "" {
		log.Fatalf("at least one of -github-owner-filter and -github-topic-filter must be set")
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
	if strings.HasSuffix(filepath, ".json") {
		tokenmap := make(map[string]string)
		err = json.Unmarshal(b, &tokenmap)
		if err != nil {
			return tokens, err
		}
		for _, token := range tokenmap {
			tokens = append(tokens, token)
		}
	} else {
		lines := strings.Split(string(b), "\n")
		for _, token := range lines {
			if token = strings.TrimSpace(token); token != "" {
				tokens = append(tokens, token)
			}
		}
	}

	return tokens, nil
}

// setEnvironmentFromJSONFile loads a JSON object from `filename` and updates the
// runtime environment with keys and values from this object using `os.Setenv`.
// Keys will be uppercased and `-` (dashes) will be replaced with `_` (underscores).
func setEnvironmentFromJSONFile(prefix, filename string) error {
	vars := make(map[string]string)
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	json.Unmarshal(b, &vars)
	if err != nil {
		return fmt.Errorf("while parsing file '%s': %w", filename, err)
	}
	for k, v := range vars {
		k = prefix + k
		k = strings.ToUpper(k)
		k = strings.ReplaceAll(k, "-", "_")
		if !patternEnvVarName.MatchString(k) {
			log.Printf("warn: env var with key '%s' does not conform to pattern '%s'. skipping!", k, patternEnvVarName)
			continue
		}
		os.Setenv(k, v)
		log.Printf("info: setting var '%s' from file '%s'", k, filename)
	}
	return nil
}
