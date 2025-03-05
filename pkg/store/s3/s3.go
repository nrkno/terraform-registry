// SPDX-FileCopyrightText: 2024 - 2025 NRK
//
// SPDX-License-Identifier: MIT

package s3

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

// S3API defines the subset of S3 client methods used by S3Store
type S3API interface {
	ListObjectsV2WithContext(ctx aws.Context, input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error)
	HeadObjectWithContext(ctx aws.Context, input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error)
}

// S3StoreInterface defines the interface for S3Store
type S3StoreInterface interface {
	ListModuleVersions(ctx context.Context, namespace, name, system string) ([]*core.ModuleVersion, error)
	GetModuleVersion(ctx context.Context, namespace, name, system, version string) (*core.ModuleVersion, error)
}

// S3Store implements S3StoreInterface
type S3Store struct {
	client s3iface.S3API
	cache  map[string][]*core.ModuleVersion
	region string
	bucket string
	logger *zap.Logger
	mut    sync.Mutex
}

func NewS3Store(client s3iface.S3API, region string, bucket string, logger *zap.Logger) *S3Store {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &S3Store{
		client: client,
		cache:  make(map[string][]*core.ModuleVersion),
		region: region,
		bucket: bucket,
		logger: logger,
	}
}

func (s *S3Store) ListModuleVersions(ctx context.Context, namespace, name, system string) ([]*core.ModuleVersion, error) {
	addr := filepath.Join(namespace, name, system)
	vers, err := s.fetchModuleVersions(ctx, addr)
	if err != nil {
		return nil, err
	}

	return vers, nil
}

func (s *S3Store) fetchModuleVersions(ctx context.Context, address string) ([]*core.ModuleVersion, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	p := address + "/"
	in := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(p),
	}
	out, err := s.client.ListObjectsV2WithContext(ctx, in)
	if err != nil {
		return nil, err
	}

	vers := make([]*core.ModuleVersion, 0)
	for _, o := range out.Contents {
		path := o.Key
		if isValidModuleSourcePath(*path) {
			vers = append(vers, &core.ModuleVersion{
				Version:   strings.Split(*path, "/")[3],
				SourceURL: fmt.Sprintf("s3::https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, *path),
			})
		}
	}

	s.cache[address] = vers

	return vers, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, system, version string) (*core.ModuleVersion, error) {
	addr := filepath.Join(namespace, name, system)
	ver, err := s.fetchModuleVersion(ctx, addr, version)
	if err != nil {
		return nil, err
	}

	return ver, nil
}

func (s *S3Store) fetchModuleVersion(ctx context.Context, address, version string) (*core.ModuleVersion, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	vers := s.cache[address]
	for _, o := range vers {
		if o.Version == version {
			return o, nil
		}
	}

	path := path.Join(address, version)
	keySuffix := version + ".zip"
	if !isValidModuleSourcePath(path) {
		s.logger.Warn("invalid module path requested: " + path)
		return nil, fmt.Errorf("module version path '%s' is not valid", path)
	}
	_, err := s.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path + "/" + keySuffix),
	})
	if err != nil {
		return nil, err
	}

	ver := &core.ModuleVersion{
		Version:   version,
		SourceURL: fmt.Sprintf("s3::https://%s.s3.%s.amazonaws.com/%s/%s", s.bucket, s.region, path, keySuffix),
	}

	s.cache[address] = append(vers, ver)

	return ver, nil
}

func isValidModuleSourcePath(path string) bool {
	// https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
	verRegExp := `(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`
	addrRegExp := `\w+/\w+/\w+`
	r := regexp.MustCompile("^" + addrRegExp + "/" + verRegExp)
	return r.MatchString(path)
}
