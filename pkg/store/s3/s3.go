package s3

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

type S3Store struct {
	client *s3.S3
	cache  map[string][]*core.ModuleVersion

	bucket string

	logger *zap.Logger
}

func NewS3Store(bucket string, logger *zap.Logger) (*S3Store, error) {
	store := &S3Store{}

	if logger == nil {
		logger = zap.NewNop()
	}

	sess, err := session.NewSession()
	if err != nil {
		return store, fmt.Errorf("session creation failed: %s", err)
	}
	logger.Debug("session created successfully")

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		return store, fmt.Errorf("session credentials not found: %s", err)
	}
	logger.Debug("session credentials found")

	store = &S3Store{
		client: s3.New(sess),
		cache:  make(map[string][]*core.ModuleVersion),
		bucket: bucket,
	}
	logger.Debug("s3 client created successfully")

	return store, nil
}

func (s *S3Store) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	moduleVersions, err := s.listModuleVersions(namespace, name, provider, "")
	if err != nil {
		return moduleVersions, err
	}
	return moduleVersions, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	moduleVersions, err := s.listModuleVersions(namespace, name, provider, version)
	moduleVersion := &core.ModuleVersion{}
	if err != nil {
		return moduleVersion, err
	}
	if len(moduleVersions) > 0 {
		moduleVersion = moduleVersions[0]
	}
	return moduleVersion, nil
}

func (s *S3Store) ReloadCache(ctx context.Context) error {
	return nil
}

func (s *S3Store) listModuleVersions(namespace, name, provider, version string) ([]*core.ModuleVersion, error) {
	moduleVersions := make([]*core.ModuleVersion, 0)

	maxKeys := aws.Int64(100)
	if version != "" {
		maxKeys = aws.Int64(1)
	}

	moduleLocation := filepath.Join(s.bucket, namespace, name, provider, version)
	in := &s3.ListObjectsV2Input{
		Bucket:  &moduleLocation,
		MaxKeys: maxKeys,
	}
	out, err := s.client.ListObjectsV2(in)
	if err != nil {
		return moduleVersions, err
	}

	keyCount := *out.KeyCount
	if keyCount == 0 {
		return moduleVersions, nil
	}

	contents := out.Contents
	for _, c := range contents {
		moduleVersion := c.Key
		moduleVersionLocation := filepath.Join(moduleLocation, *moduleVersion)
		moduleVersions = append(moduleVersions, &core.ModuleVersion{
			Version:   *moduleVersion,
			SourceURL: createModuleSrcURL(moduleVersionLocation, "zip"),
		})
	}

	return moduleVersions, nil
}

func createModuleSrcURL(location, extension string) string {
	location = strings.TrimSuffix(location, "/")
	extension = strings.TrimPrefix(extension, ".")
	return fmt.Sprintf("s3::%s.%s", location, extension)
}
