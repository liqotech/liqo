// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"context"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func (o *Options) cloneRepository(ctx context.Context) error {
	var err error
	o.tmpDir, err = os.MkdirTemp("", "liqo-*")
	if err != nil {
		return err
	}

	repo, err := git.PlainCloneContext(ctx, o.tmpDir, false, &git.CloneOptions{URL: o.RepoURL})
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = worktree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(o.Version)})
	if err != nil {
		return err
	}

	o.ChartPath = path.Join(o.tmpDir, "deployments", "liqo")
	return nil
}
