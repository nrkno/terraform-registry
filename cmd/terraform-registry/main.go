// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nrkno/terraform-registry/pkg/registry"
	"github.com/nrkno/terraform-registry/pkg/store/github"
	"go.uber.org/zap"
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
	logLevelStr    string
	logFormatStr   string

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

	logger *zap.Logger = zap.NewNop()
)

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":8080", "")
	flag.BoolVar(&authDisabled, "auth-disabled", false, "")
	flag.StringVar(&authTokensFile, "auth-tokens-file", "", "")
	flag.StringVar(&envJSONFiles, "env-json-files", "", "List of comma-separated paths to JSON files. Converts the keys to uppercase and replaces all '-' with '_'. Prefix filepaths with 'myprefix:' to use a prefix names of the variables from a specific file.")
	flag.BoolVar(&tlsEnabled, "tls-enabled", false, "")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "", "")
	flag.StringVar(&storeType, "store", "", "Store backend to use (choices: github)")
	flag.StringVar(&logLevelStr, "log-level", "info", "Levels: debug, info, warn, error")
	flag.StringVar(&logFormatStr, "log-format", "console", "Formats: json, console")

	flag.StringVar(&gitHubOwnerFilter, "github-owner-filter", "", "GitHub org/user repository filter")
	flag.StringVar(&gitHubTopicFilter, "github-topic-filter", "", "GitHub topic repository filter")

}

func main() {
	flag.Parse()

	if len(os.Args[1:]) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Configure logging
	logConfig := zap.NewProductionConfig()
	logLevel, err := zap.ParseAtomicLevel(logLevelStr)
	if err != nil {
		panic(fmt.Sprintf("zap logger: %v", err))
	}
	logConfig.Level = logLevel
	logConfig.Encoding = logFormatStr
	logger, err = logConfig.Build()
	if err != nil {
		panic(fmt.Sprintf("zap logger: %v", err))
	}
	defer logger.Sync()

	// Load environment from files
	for _, item := range strings.Split(envJSONFiles, ",") {
		if len(item) == 0 {
			continue
		}

		prefix := ""
		filename := item
		// If the filename is prefixed, the prefix must be separated from the filename.
		if split := strings.SplitN(filename, ":", 2); len(split) == 2 {
			prefix = split[0]
			filename = split[1]
		}
		if err := setEnvironmentFromJSONFile(prefix, filename); err != nil {
			logger.Fatal("failed to load environment from file(s)",
				zap.Errors("err", []error{err}),
			)
		}
	}

	// Load environment variables here!
	gitHubToken = os.Getenv("GITHUB_TOKEN")

	reg := registry.NewRegistry(logger)
	reg.IsAuthDisabled = authDisabled

	// Configure authentication
	if !reg.IsAuthDisabled {
		tokens, err := parseAuthTokensFile(authTokensFile)
		if err != nil {
			logger.Fatal("failed to load auth tokens",
				zap.Errors("err", []error{err}),
			)
		}
		if len(tokens) == 0 {
			logger.Warn("no tokens found in auth token file")
		}
		reg.SetAuthTokens(tokens)

		logger.Info("authentication enabled")
		logger.Info("loaded auth tokens", zap.Int("count", len(tokens)))
	} else {
		logger.Warn("authentication disabled")
	}

	// Configure the chosen store type
	switch storeType {
	case "github":
		gitHubRegistry(reg)
	default:
		logger.Fatal("invalid store type", zap.String("selected", storeType))
	}

	srv := http.Server{
		Addr:              listenAddr,
		Handler:           reg,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       60 * time.Second, // keep-alive timeout
	}

	logger.Info("starting HTTP server",
		zap.Bool("tls", tlsEnabled),
		zap.String("listenAddr", listenAddr),
	)
	if tlsEnabled {
		logger.Panic("ListenAndServe",
			zap.Errors("err", []error{srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile)}),
		)
	} else {
		logger.Panic("ListenAndServe",
			zap.Errors("err", []error{srv.ListenAndServe()}),
		)
	}
}

// gitHubRegistry configures the registry to use GitHubStore.
func gitHubRegistry(reg *registry.Registry) {
	if gitHubToken == "" {
		logger.Fatal("missing environment var GITHUB_TOKEN")
	}
	if gitHubOwnerFilter == "" && gitHubTopicFilter == "" {
		logger.Fatal("at least one of -github-owner-filter and -github-topic-filter must be set")
	}

	store := github.NewGitHubStore(gitHubOwnerFilter, gitHubTopicFilter, gitHubToken, logger.Named("github store"))
	reg.SetModuleStore(store)

	// Fill store cache initially
	logger.Debug("loading GitHub store cache")
	if err := store.ReloadCache(context.Background()); err != nil {
		logger.Error("failed to load GitHub store cache",
			zap.Errors("err", []error{err}),
		)
	}

	// Reload store cache on regular intervals
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		<-t.C // ignore the first tick

		for {
			logger.Debug("reloading GitHub store cache")
			if err := store.ReloadCache(context.Background()); err != nil {
				logger.Error("failed to reload GitHub store cache",
					zap.Errors("err", []error{err}),
				)
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
			logger.Warn("unexpected environment variable name format",
				zap.String("expected pattern", patternEnvVarName.String()),
				zap.String("name", k),
			)
			continue
		}
		os.Setenv(k, v)
		logger.Info("environment variable set from file",
			zap.String("name", k),
			zap.String("file", filename),
		)
	}
	return nil
}
