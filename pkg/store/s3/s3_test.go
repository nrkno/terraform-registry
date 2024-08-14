// SPDX-FileCopyrightText: 2024 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package s3

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/matryer/is"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockS3API is a mock implementation for the S3API interface
type MockS3API struct {
	mock.Mock
	s3iface.S3API
}

func (m *MockS3API) ListObjectsV2WithContext(ctx aws.Context, input *s3.ListObjectsV2Input, opts ...request.Option) (*s3.ListObjectsV2Output, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

func (m *MockS3API) HeadObjectWithContext(ctx aws.Context, input *s3.HeadObjectInput, opts ...request.Option) (*s3.HeadObjectOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)
	mockS3 := new(MockS3API)

	store := NewS3Store(mockS3, "us-east-1", "mytestbucket", zap.NewNop())

	mockS3.On("ListObjectsV2WithContext", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []*s3.Object{
			{Key: aws.String("testnamespace/testname/testprovider/1.0.0/1.0.0.zip")},
			{Key: aws.String("testnamespace/testname/testprovider/1.1.1/1.1.1.zip")},
		},
	}, nil)

	vers, err := store.ListModuleVersions(context.Background(), "testnamespace", "testname", "testprovider")
	is.True(err == nil)
	is.Equal(len(vers), 2)
	is.Equal(vers[0].SourceURL, "s3::https://mytestbucket.s3.us-east-1.amazonaws.com/testnamespace/testname/testprovider/1.0.0/1.0.0.zip")
	is.Equal(vers[1].SourceURL, "s3::https://mytestbucket.s3.us-east-1.amazonaws.com/testnamespace/testname/testprovider/1.1.1/1.1.1.zip")

	mockS3.AssertExpectations(t)
}

func TestGetModuleVersion(t *testing.T) {
	mockS3 := new(MockS3API)

	store := NewS3Store(mockS3, "us-east-1", "mytestbucket", zap.NewNop())

	t.Run("returns matching version", func(t *testing.T) {
		is := is.New(t)
		mockS3.On("HeadObjectWithContext", mock.Anything, mock.Anything).Return(&s3.HeadObjectOutput{}, nil)
		ver, err := store.GetModuleVersion(context.Background(), "testnamespace", "testname", "testprovider", "1.0.0")
		is.True(err == nil)
		is.Equal(ver.Version, "1.0.0")
		is.Equal(ver.SourceURL, "s3::https://mytestbucket.s3.us-east-1.amazonaws.com/testnamespace/testname/testprovider/1.0.0/1.0.0.zip")
	})

	t.Run("invalid version", func(t *testing.T) {
		is := is.New(t)
		ver, err := store.GetModuleVersion(context.Background(), "test-owner", "test-repo", "generic", "1.0.0")
		is.True(err != nil)
		is.True(ver == nil)
		is.Equal(err.Error(), "module version path 'test-owner/test-repo/generic/1.0.0' is not valid")
	})
}
