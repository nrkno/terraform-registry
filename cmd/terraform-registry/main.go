// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/nrkno/terraform-registry/pkg/registry"
	"github.com/nrkno/terraform-registry/pkg/store/github"
	"github.com/nrkno/terraform-registry/pkg/store/s3"
	"go.uber.org/zap"
)

var (
	listenAddr            string
	accessLogDisabled     bool
	accessLogIgnoredPaths string
	authDisabled          bool
	authTokensFile        string
	envJSONFiles          string
	tlsEnabled            bool
	tlsCertFile           string
	tlsKeyFile            string
	storeType             string
	logLevelStr           string
	logFormatStr          string
	printVersionInfo      bool

	S3Region string
	S3Bucket string

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

	// These variables are set at build time using ldflags.
	version   = "(devel)"
	buildDate = "unknown"

	logger *zap.Logger = zap.NewNop()
)

const (
	developmentVersion = "(devel)"
	programName        = "terraform-registry"
)

func init() {
	flag.StringVar(&listenAddr, "listen-addr", ":8080", "")
	flag.BoolVar(&accessLogDisabled, "access-log-disabled", false, "")
	flag.StringVar(&accessLogIgnoredPaths, "access-log-ignored-paths", "", "Comma-separated list of request paths to ignore logging for")
	flag.BoolVar(&authDisabled, "auth-disabled", false, "")
	flag.StringVar(&authTokensFile, "auth-tokens-file", "", "JSON encoded file containing a map of auth token descriptions and tokens.")
	flag.StringVar(&envJSONFiles, "env-json-files", "", "Comma-separated list of paths to JSON encoded files containing a map of environment variable names and values to set. Converts the keys to uppercase and replaces all occurences of '-' with '_'. E.g. prefix filepaths with 'myprefix_:' to prefix all keys in the file with 'MYPREFIX_' before they are set.")
	flag.BoolVar(&tlsEnabled, "tls-enabled", false, "")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "", "")
	flag.StringVar(&storeType, "store", "", "Store backend to use (choices: github, s3)")
	flag.StringVar(&logLevelStr, "log-level", "info", "Levels: debug, info, warn, error")
	flag.StringVar(&logFormatStr, "log-format", "console", "Formats: json, console")
	flag.BoolVar(&printVersionInfo, "version", false, "Print version info and exit")

	flag.StringVar(&gitHubOwnerFilter, "github-owner-filter", "", "GitHub org/user repository filter")
	flag.StringVar(&gitHubTopicFilter, "github-topic-filter", "", "GitHub topic repository filter")

	flag.StringVar(&S3Region, "s3-region", "", "S3 region such as us-east-1")
	flag.StringVar(&S3Bucket, "s3-bucket", "", "S3 bucket name")
}

func main() {
	flag.Parse()

	if len(os.Args[1:]) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if printVersionInfo {
		fmt.Printf("%s %s\n", programName, versionString())
		os.Exit(0)
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
	reg.AccessLogIgnoredPaths = strings.Split(accessLogIgnoredPaths, ",")
	reg.IsAccessLogDisabled = accessLogDisabled
	reg.IsAuthDisabled = authDisabled

	logger.Info("HTTP access log configuration", zap.Bool("disabled", reg.IsAccessLogDisabled), zap.Strings("ignoredPaths", reg.AccessLogIgnoredPaths))

	// Configure authentication
	if !reg.IsAuthDisabled {
		if authTokensFile == "" {
			logger.Fatal("-auth-tokens-file is not set. Provide a valid path or set -auth-disabled.")
		}

		// Watch for changes of the auth file
		go watchFile(context.TODO(), authTokensFile, 10*time.Second, func(b []byte) {
			tokens, err := parseAuthTokens(b)
			if err != nil {
				logger.Error("failed to load auth tokens",
					zap.Errors("err", []error{err}),
				)
			}

			reg.SetAuthTokens(tokens)

			if len(tokens) == 0 {
				logger.Warn("reloaded auth token file", zap.Int("count", len(tokens)))
			} else {
				logger.Info("reloaded auth token file", zap.Int("count", len(tokens)))
			}
		})
		logger.Info("authentication enabled")
	} else {
		logger.Warn("authentication disabled")
	}

	// Configure the chosen store type
	switch storeType {
	case "github":
		gitHubRegistry(reg)
	case "s3":
		s3Registry(reg)
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

// watchFile reads the contents of the file at `filename`, first immediately, then at at every `interval`.
// If and only if the file contents have changed since the last invocation of `callback` it is called again.
// Note that the callback will always be called initially when this function is called.
func watchFile(ctx context.Context, filename string, interval time.Duration, callback func(b []byte)) {
	var lastSum []byte
	h := sha1.New()

	fn := func() {
		b, err := os.ReadFile(filename)
		if err != nil {
			logger.Error("watchFile: failed to read file",
				zap.String("filename", filename),
				zap.Errors("err", []error{err}),
			)
			return
		}
		if sum := h.Sum(b); bytes.Equal(sum, lastSum) {
			logger.Debug("watchFile: file contents unchanged. do nothing.",
				zap.String("filename", filename),
			)
			return
		} else {
			logger.Debug("watchFile: file contents updated. triggering callback.",
				zap.String("filename", filename),
			)
			callback(b)
			lastSum = sum
		}
	}

	// Don't wait for the first tick
	fn()

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("watchFile: goroutine stopped",
				zap.String("filename", filename),
				zap.Errors("err", []error{ctx.Err()}),
			)
			return
		case <-t.C:
			fn()
		}
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

// s3Registry configures the registry to use S3Store.
func s3Registry(reg *registry.Registry) {
	if S3Region == "" {
		logger.Fatal("Missing flag '-s3-region'")
	}
	if S3Bucket == "" {
		logger.Fatal("Missing flag '-s3-bucket'")
	}

	sess, err := session.NewSession()
	if err != nil {
		logger.Fatal("AWS session creation failed")
	}
	logger.Debug("AWS session created successfully")

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		logger.Fatal("AWS session credentials not found")
	}
	s3Sess := awss3.New(sess)

	store := s3.NewS3Store(s3Sess, S3Region, S3Bucket, logger.Named("s3 store"))
	if err != nil {
		logger.Fatal("failed to create S3 store",
			zap.Errors("err", []error{err}),
		)
	}
	reg.SetModuleStore(store)
}

// parseAuthTokens returns a map of all elements in the JSON object contained in `b`.
func parseAuthTokens(b []byte) (map[string]string, error) {
	tokens := make(map[string]string)
	if err := json.Unmarshal(b, &tokens); err != nil {
		return nil, err
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

// versionString returns a string with version information for this program,
// like `v5.0.4-0.20230601165947-6ce0bf390ce3 linux amd64` for release builds,
// or `(devel).unknown-478ce46fb3ab76445001e614fec7ff1dd0c6cfe0 linux amd64` for local builds.
func versionString() string {
	v := struct {
		Version   string
		BuildDate string
		GitCommit string
		GoArch    string
		GoOS      string
		GoVersion string
	}{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: "unknown",
		GoArch:    runtime.GOARCH,
		GoOS:      runtime.GOOS,
		GoVersion: runtime.Version(),
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				v.GitCommit = setting.Value
			}
		}
	}

	return fmt.Sprintf("%s.%s-%s %s %s", v.Version, v.BuildDate, v.GitCommit, v.GoOS, v.GoArch)
}
