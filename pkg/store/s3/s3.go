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
	moduleVersions, err := s.genModuleVersionList(namespace, name, provider, "")
	if err != nil {
		return moduleVersions, err
	}
	return moduleVersions, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	moduleVersions, err := s.genModuleVersionList(namespace, name, provider, version)
	moduleVersion := moduleVersions[0]
	if err != nil {
		return moduleVersion, err
	}
	return moduleVersion, nil
}

func (s *S3Store) ReloadCache(ctx context.Context) error {
	return nil
}

func (s *S3Store) genModuleVersionList(namespace, name, provider, version string) ([]*core.ModuleVersion, error) {
	versions := make([]*core.ModuleVersion, 0)

	maxKeys := aws.Int64(100)
	if version == "" {
		maxKeys = aws.Int64(1)
	}

	modulePath := filepath.Join(s.bucket, namespace, name, provider, version)
	in := &s3.ListObjectsV2Input{
		Bucket:  &modulePath,
		MaxKeys: maxKeys,
	}
	out, err := s.client.ListObjectsV2(in)
	if err != nil {
		return versions, err
	}

	keyCount := *out.KeyCount
	if keyCount == 0 {
		return versions, nil
	}

	contents := out.Contents
	for _, o := range contents {
		version := o.Key
		moduleVersionPath := filepath.Join(modulePath, *version)
		versions = append(versions, &core.ModuleVersion{
			Version:   *version,
			SourceURL: genModSrcURL(moduleVersionPath, "zip"),
		})
	}

	return versions, nil
}

func genModSrcURL(location, extension string) string {
	location = strings.TrimSuffix(location, "/")
	extension = strings.TrimPrefix(extension, ".")
	return fmt.Sprintf("s3::%s.%s", location, extension)
}
