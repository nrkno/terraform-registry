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
	s := &S3Store{}

	if logger == nil {
		logger = zap.NewNop()
	}

	sess, err := session.NewSession()
	if err != nil {
		return s, fmt.Errorf("session creation failed: %s", err)
	}
	logger.Debug("session created successfully")

	_, err = sess.Config.Credentials.Get()
	if err != nil {
		return s, fmt.Errorf("session credentials not found: %s", err)
	}
	logger.Debug("session credentials found")

	s = &S3Store{
		client: s3.New(sess),
		cache:  make(map[string][]*core.ModuleVersion),
		bucket: bucket,
	}
	logger.Debug("s3 client created successfully")

	return s, nil
}

func (s *S3Store) ListModuleVersions(ctx context.Context, namespace, name, system string) ([]*core.ModuleVersion, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	mvs := make([]*core.ModuleVersion, 0)
	mp := filepath.Join(namespace, name, system)

	out, err := s.client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:     aws.String(s.bucket),
		Prefix:     aws.String(mp),
		StartAfter: aws.String(mp + "/"),
	})
	if err != nil {
		return mvs, err
	}

	kc := *out.KeyCount
	if kc == 0 {
		return mvs, nil
	}

	cs := out.Contents
	for _, o := range cs {
		k := o.Key
		if strings.HasSuffix(*k, "/") {
			v := strings.TrimPrefix(*k, mp+"/")
			v = strings.TrimSuffix(v, "/")
			mvs = append(mvs, &core.ModuleVersion{
				Version:   v,
				SourceURL: fmt.Sprintf("s3::https://s3.amazonaws.com/%s/%s.zip", *k, v),
			})
		}
	}

	return mvs, nil
}

func (s *S3Store) GetModuleVersion(ctx context.Context, namespace, name, system, version string) (*core.ModuleVersion, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	mv := &core.ModuleVersion{}
	mp := filepath.Join(namespace, name, system, version)

	out, err := s.client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:     aws.String(s.bucket),
		Prefix:     aws.String(mp),
		StartAfter: aws.String(mp + "/"),
	})
	if err != nil {
		return mv, err
	}

	kc := *out.KeyCount
	if kc == 0 {
		return mv, nil
	}

	k := *out.Contents[0].Key
	if strings.HasSuffix(k, ".zip") {
		mv = &core.ModuleVersion{
			Version:   version,
			SourceURL: fmt.Sprintf("s3::https://s3.amazonaws.com/%s/%s.zip", mp, version),
		}
	}

	return mv, nil
}

func (s *S3Store) ReloadCache(ctx context.Context) error {
	return nil
}
