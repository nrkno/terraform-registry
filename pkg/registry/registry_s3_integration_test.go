// SPDX-FileCopyrightText: 2026 NRK
//
// SPDX-License-Identifier: MIT

package registry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/matryer/is"
	storeS3 "github.com/nrkno/terraform-registry/pkg/store/s3"
	"go.uber.org/zap"
)

type fakeS3Client struct {
	listOutput *awss3.ListObjectsV2Output
	listErr    error
	headOutput *awss3.HeadObjectOutput
	headErr    error

	lastListInput *awss3.ListObjectsV2Input
	lastHeadInput *awss3.HeadObjectInput
}

func (f *fakeS3Client) ListObjectsV2(_ context.Context, input *awss3.ListObjectsV2Input, _ ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error) {
	f.lastListInput = input
	return f.listOutput, f.listErr
}

func (f *fakeS3Client) HeadObject(_ context.Context, input *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	f.lastHeadInput = input
	return f.headOutput, f.headErr
}

func TestS3ModuleRoutesIntegration(t *testing.T) {
	t.Run("lists module versions from s3-backed store", func(t *testing.T) {
		is := is.New(t)

		fake := &fakeS3Client{
			listOutput: &awss3.ListObjectsV2Output{
				Contents: []types.Object{
					{Key: aws.String("testnamespace/testname/testprovider/1.0.0/1.0.0.zip")},
					{Key: aws.String("testnamespace/testname/testprovider/1.1.1/1.1.1.zip")},
				},
			},
		}

		moduleStore := storeS3.NewS3Store(fake, "us-east-1", "mytestbucket", zap.NewNop())
		reg := Registry{
			IsAuthDisabled: true,
			moduleStore:    moduleStore,
			logger:         zap.NewNop(),
		}
		reg.setupRoutes()

		req := httptest.NewRequest("GET", "/v1/modules/testnamespace/testname/testprovider/versions", nil)
		w := httptest.NewRecorder()
		reg.router.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		is.NoErr(err)

		is.Equal(resp.StatusCode, http.StatusOK)
		is.Equal(resp.Header.Get("Content-Type"), "application/json")
		is.True(strings.Contains(string(body), `"1.0.0"`))
		is.True(strings.Contains(string(body), `"1.1.1"`))

		is.True(fake.lastListInput != nil)
		is.Equal(*fake.lastListInput.Bucket, "mytestbucket")
		is.Equal(*fake.lastListInput.Prefix, "testnamespace/testname/testprovider/")
	})

	t.Run("returns terraform download header for existing module version", func(t *testing.T) {
		is := is.New(t)

		fake := &fakeS3Client{
			headOutput: &awss3.HeadObjectOutput{},
		}

		moduleStore := storeS3.NewS3Store(fake, "us-east-1", "mytestbucket", zap.NewNop())
		reg := Registry{
			IsAuthDisabled: true,
			moduleStore:    moduleStore,
			logger:         zap.NewNop(),
		}
		reg.setupRoutes()

		req := httptest.NewRequest("GET", "/v1/modules/testnamespace/testname/testprovider/1.0.0/download", nil)
		w := httptest.NewRecorder()
		reg.router.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		is.Equal(resp.StatusCode, http.StatusNoContent)
		is.Equal(resp.Header.Get("X-Terraform-Get"), "s3::https://mytestbucket.s3.us-east-1.amazonaws.com/testnamespace/testname/testprovider/1.0.0/1.0.0.zip")

		is.True(fake.lastHeadInput != nil)
		is.Equal(*fake.lastHeadInput.Bucket, "mytestbucket")
		is.Equal(*fake.lastHeadInput.Key, "testnamespace/testname/testprovider/1.0.0/1.0.0.zip")
	})

	t.Run("maps s3 head errors to not found response", func(t *testing.T) {
		is := is.New(t)

		fake := &fakeS3Client{
			headErr: errors.New("head object failed"),
		}

		moduleStore := storeS3.NewS3Store(fake, "us-east-1", "mytestbucket", zap.NewNop())
		reg := Registry{
			IsAuthDisabled: true,
			moduleStore:    moduleStore,
			logger:         zap.NewNop(),
		}
		reg.setupRoutes()

		req := httptest.NewRequest("GET", "/v1/modules/testnamespace/testname/testprovider/1.0.0/download", nil)
		w := httptest.NewRecorder()
		reg.router.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		is.Equal(resp.StatusCode, http.StatusNotFound)
	})
}
