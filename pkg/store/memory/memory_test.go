// SPDX-FileCopyrightText: 2022 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"context"
	"testing"

	"github.com/matryer/is"
	"github.com/nrkno/terraform-registry/pkg/core"
)

func TestGet(t *testing.T) {
	is := is.New(t)

	v := &core.ModuleVersion{
		Version: "1",
	}
	s := NewMemoryStore()
	s.store["foo"] = []*core.ModuleVersion{v}

	res := s.Get("foo")
	is.Equal(len(res), 1)
	is.Equal(res[0], v)
}

func TestSet(t *testing.T) {
	is := is.New(t)

	v := &core.ModuleVersion{
		Version: "1",
	}
	s := NewMemoryStore()
	s.Set("foo", []*core.ModuleVersion{v})

	res := s.Get("foo")
	is.Equal(len(res), 1)
	is.Equal(res[0], v)
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)

	s := NewMemoryStore()
	s.Set("foo/bar/baz", []*core.ModuleVersion{
		{Version: "1"},
		{Version: "2"},
		{Version: "3"},
	})

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

	s := NewMemoryStore()
	s.Set("foo/bar/baz", []*core.ModuleVersion{
		{Version: "1", SourceURL: "https://example.com/foo/bar/baz/v1.tar.gz"},
		{Version: "2", SourceURL: "https://example.com/foo/bar/baz/v2.tar.gz"},
		{Version: "3", SourceURL: "https://example.com/foo/bar/baz/v3.tar.gz"},
	})

	t.Run("returns matching version", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "foo", "bar", "baz", "2")
		is.NoErr(err)
		is.Equal(ver.Version, "2")
		is.Equal(ver.SourceURL, "https://example.com/foo/bar/baz/v2.tar.gz")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "foo", "bar", "baz", "13")
		is.True(err != nil)
		is.Equal(ver, nil)
	})
}
