package s3

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

type S3Store struct {
	client *s3.S3
	cache  map[string][]*core.ModuleVersion

	bucketName string
	prefix     string

	logger *zap.Logger
}

func NewS3Store(bucketName string, prefix string, logger *zap.Logger) (*S3Store, error) {
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
		client:     s3.New(sess),
		cache:      make(map[string][]*core.ModuleVersion),
		bucketName: bucketName,
		prefix:     prefix,
	}

	return store, nil
}

func (s *S3Store) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]*core.ModuleVersion, error) {
	modulePath := filepath.Join(s.bucketName, namespace, name, provider)
	versions := make([]*core.ModuleVersion, 0)
	input := &s3.ListObjectsV2Input{
		Bucket: &modulePath,
		Prefix: &s.prefix,
	}
	output, err := s.client.ListObjectsV2(input)
	if err != nil {
		return versions, err
	}
	contents := output.Contents
	region := *s.client.Config.Region
	for _, v := range contents {
		version := v.Key
		moduleVersionPath := filepath.Join(modulePath, *version)
		versions = append(versions, &core.ModuleVersion{
			Version:   *version,
			SourceURL: genSourceURL(region, moduleVersionPath),
		})
	}

	return versions, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, provider, version string) (*core.ModuleVersion, error) {
	v := &core.ModuleVersion{}
	moduleVersionPath := filepath.Join(s.bucketName, namespace, name, provider, version)
	input := &s3.ListObjectsV2Input{
		Bucket: &moduleVersionPath,
		Prefix: &s.prefix,
	}
	output, err := s.client.ListObjectsV2(input)
	if err != nil {
		return v, err
	}
	contents := output.Contents
	region := *s.client.Config.Region
	if len(contents) == 0 {
		return v, err
	}
	v = &core.ModuleVersion{
		Version:   version,
		SourceURL: genSourceURL(region, moduleVersionPath),
	}
	return v, nil
}

func (s *S3Store) PublishModuleVersion() {

}

func (s *S3Store) UnpublishModuleVersion() {

}

func (s *S3Store) ReloadCache(ctx context.Context) error {
	// Traverse all modules with directory structure
	return nil
}

func genSourceURL(region string, path string) string {
	path = strings.TrimPrefix(path, "/")
	return fmt.Sprintf("s3::https://s3-%s.amazonaws.com/%s/module.zip", region, path)
}
