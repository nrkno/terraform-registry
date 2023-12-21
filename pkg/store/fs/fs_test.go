// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package fs

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/matryer/is"
)

func TestGet(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "tfreg-*")
	is.NoErr(err)

	os.MkdirAll(path.Join(root, "foo/1.0.0"), 0777)

	s := NewFSStore(root)

	res := s.Get("foo")
	is.Equal(len(res), 1)
	is.Equal(res[0].Version, "1.0.0")
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "tfreg-*")
	is.NoErr(err)
	defer os.RemoveAll(root)

	os.MkdirAll(path.Join(root, "foo/bar/baz/1"), 0777)
	os.MkdirAll(path.Join(root, "foo/bar/baz/2"), 0777)
	os.MkdirAll(path.Join(root, "foo/bar/baz/3"), 0777)

	//os.WriteFile(path.Join(root, "foo/bar/baz/1", "v1.zip"), []byte(""), 0666)
	//os.WriteFile(path.Join(root, "foo/bar/baz/2", "v2.zip"), []byte(""), 0666)
	//os.WriteFile(path.Join(root, "foo/bar/baz/3", "v3.zip"), []byte(""), 0666)

	s := NewFSStore(root)
	t.Run("returns list of versions", func(t *testing.T) {
		is := is.New(t)
		versions, err := s.ListModuleVersions(context.TODO(), "foo", "bar", "baz")
		is.NoErr(err)
		is.Equal(len(versions), 3)
		is.Equal(versions[1].Version, "2")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		versions, err := s.ListModuleVersions(context.TODO(), "wrong", "wrong", "wrong")
		is.True(err != nil)
		is.Equal(versions, nil)
	})
}

func TestGetModuleVersion(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "tfreg-*")
	is.NoErr(err)
	defer os.RemoveAll(root)

	os.MkdirAll(path.Join(root, "foo/bar/baz/1"), 0777)
	os.MkdirAll(path.Join(root, "foo/bar/baz/2"), 0777)
	os.MkdirAll(path.Join(root, "foo/bar/baz/3"), 0777)

	s := NewFSStore(root)

	t.Run("returns matching version", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "foo", "bar", "baz", "2")
		is.NoErr(err)
		is.Equal(ver.Version, "2")

		// Reminder to update tests when we populate SourceURL later on
		is.Equal(ver.SourceURL, "")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "foo", "bar", "baz", "13")
		is.True(err != nil)
		is.Equal(ver, nil)
	})
}
