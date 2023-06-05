// SPDX-FileCopyrightText: 2022 NRK
// SPDX-FileCopyrightText: 2023 NRK
//
// SPDX-License-Identifier: GPL-3.0-only

package fs

import (
	"context"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/matryer/is"
	"github.com/nrkno/terraform-registry/pkg/core"
	"go.uber.org/zap"
)

func TestGet(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "terraform-registry-test-*")
	is.NoErr(err)
	defer os.RemoveAll(root) // clean up

	s, err := NewStore(root, zap.NewNop())
	is.NoErr(err)

	// Tag 3 versions of a new module
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.0.0")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.2.3")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v2.0.0")

	// Get all versions for a module
	res, err := s.Get("namespace/module1/generic")
	is.NoErr(err)

	is.Equal(len(res), 3)                 // modules exposed in store
	is.Equal(res[0], &core.ModuleVersion{ // module version exposed correctly
		"v1.0.0",
		"/download/namespace/module1/v1.0.0",
	})
	is.Equal(res[1], &core.ModuleVersion{ // module version exposed correctly
		"v1.2.3",
		"/download/namespace/module1/v1.2.3",
	})
	is.Equal(res[2], &core.ModuleVersion{ // module version exposed correctly
		"v2.0.0",
		"/download/namespace/module1/v2.0.0",
	})
}

func TestListModules(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "terraform-registry-test-*")
	is.NoErr(err)
	defer os.RemoveAll(root) // clean up

	s, err := NewStore(root, zap.NewNop())
	is.NoErr(err)

	t.Run("returns an empty list of modules", func(t *testing.T) {
		is := is.New(t)
		versions, err := s.ListModules(context.TODO())
		is.NoErr(err)
		is.Equal(len(versions), 3)
		//is.Equal(versions[1].Version, "v1.2.3")
	})

	createOrUpdateModuleRepoWithTag(t, root, "namespaceA", "module1", "v1.0.0")
	createOrUpdateModuleRepoWithTag(t, root, "namespaceB", "module2", "v1.0.0")
	createOrUpdateModuleRepoWithTag(t, root, "namespaceC", "module3", "v1.0.0")

	t.Run("returns a list of modules", func(t *testing.T) {
		is := is.New(t)
		modules, err := s.ListModules(context.TODO())
		is.NoErr(err)
		is.Equal(len(modules), 3)

		is.Equal(modules[0].Namespace, "namespaceA")
		is.Equal(len(modules[0].Versions), 1)

		is.Equal(modules[1].Namespace, "namespaceB")
		is.Equal(len(modules[1].Versions), 1)

		is.Equal(modules[2].Namespace, "namespaceC")
		is.Equal(len(modules[2].Versions), 1)
	})
}

func TestListModuleVersions(t *testing.T) {
	is := is.New(t)

	root, err := os.MkdirTemp("", "terraform-registry-test-*")
	is.NoErr(err)
	defer os.RemoveAll(root) // clean up

	s, err := NewStore(root, zap.NewNop())
	is.NoErr(err)

	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.0.0")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.2.3")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v2.0.0")

	t.Run("returns list of versions", func(t *testing.T) {
		is := is.New(t)
		versions, err := s.ListModuleVersions(context.TODO(), "namespace", "module1", "generic")
		is.NoErr(err)
		is.Equal(len(versions), 3)
		is.Equal(versions[1].Version, "v1.2.3")
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

	root, err := os.MkdirTemp("", "terraform-registry-test-*")
	is.NoErr(err)
	defer os.RemoveAll(root) // clean up

	s, err := NewStore(root, zap.NewNop())
	is.NoErr(err)

	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.0.0")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v1.2.3")
	createOrUpdateModuleRepoWithTag(t, root, "namespace", "module1", "v2.0.0")

	t.Run("returns matching version", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "namespace", "module1", "generic", "v1.2.3")
		is.NoErr(err)
		is.Equal(ver.Version, "v1.2.3")
		is.Equal(ver.SourceURL, "/download/namespace/module1/v1.2.3")
	})

	t.Run("errs when missing", func(t *testing.T) {
		is := is.New(t)
		ver, err := s.GetModuleVersion(context.TODO(), "namespace", "module1", "generic", "v4.2.0")
		is.True(err != nil)
		is.Equal(ver, nil)
	})
}

func createOrUpdateModuleRepoWithTag(t *testing.T, root, namespace, name, tag string) {
	is := is.New(t)
	repoPath := path.Join(root, namespace, name)

	var repo *git.Repository
	var err error

	// Create repo if it doesn't exist
	if _, err = os.Stat(repoPath); err != nil {
		err = os.MkdirAll(repoPath, 0770)
		is.NoErr(err)

		repo, err = git.PlainInit(repoPath, false)
		is.NoErr(err)
	} else {
		repo, err = git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{
			DetectDotGit:          false,
			EnableDotGitCommonDir: false,
		})
		is.NoErr(err)
	}

	// Write something to the file. Should be unique enough with a nano timestamp.
	err = os.WriteFile(path.Join(repoPath, "testfile"), []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0660)
	is.NoErr(err)

	tree, err := repo.Worktree()
	is.NoErr(err)

	_, err = tree.Add("testfile")
	is.NoErr(err)

	hash, err := tree.Commit(tag, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	is.NoErr(err)

	_, err = repo.CreateTag(tag, hash, &git.CreateTagOptions{
		Message: tag,
		Tagger: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	is.NoErr(err)
}
