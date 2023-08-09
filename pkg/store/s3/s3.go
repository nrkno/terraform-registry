package s3

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

type S3Store struct {
	client *s3.S3
	cache  map[string][]*core.ModuleVersion

	bucket  string
	prefix  string
	profile string
	region  string

	logger *zap.Logger
}

func NewS3Store(bucket string, prefix string, region string, profile string, logger *zap.Logger) (*S3Store, error) {
	store := &S3Store{}

	if bucket == "" {
		return store, fmt.Errorf("%s", "missing parameter: 'bucket'")
	}
	if profile == "" {
		profile = "default"
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	sessOpts := session.Options{
		Profile:                 profile,
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
	}
	sess, err := session.NewSessionWithOptions(sessOpts)
	if err != nil {
		return store, fmt.Errorf("session creation failed: %s", err)
	}

	store = &S3Store{
		client:  s3.New(sess),
		cache:   make(map[string][]*core.ModuleVersion),
		bucket:  bucket,
		prefix:  prefix,
		region:  region,
		profile: profile,
		logger:  logger,
	}

	return store, nil
}

func (s *S3Store) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	versions := make([]*core.ModuleVersion, 0)
	path := filepath.Join(s.bucket, namespace, name, provider)
	input := &s3.ListObjectsV2Input{
		Bucket: &path,
		Prefix: &s.prefix,
	}
	output, err := s.client.ListObjectsV2(input)
	if err != nil {
		return versions, err
	}
	contents := output.Contents
	for _, v := range contents {
		version := v.Key
		versions = append(versions, &core.ModuleVersion{
			Version:   *version,
			SourceURL: fmt.Sprintf("s3::https://s3-%s.amazonaws.com/%s/%s/%s/%s.zip", "", "", "", "", ""),
		})
	}

	return versions, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	// Check cache. Then look up directly and update cache
	return &core.ModuleVersion{}, nil
}

func (s *S3Store) PublishModuleVersion() {

}

func (s *S3Store) UnpublishModuleVersion() {

}

func (s *S3Store) ReloadCache(ctx context.Context) error {
	// Traverse all modules with directory structure
	return nil
}
