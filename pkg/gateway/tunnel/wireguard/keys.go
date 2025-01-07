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

package wireguard

import (
	"encoding/base64"
	"io"
	"os"
	"path"
	"path/filepath"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// LoadKeys loads the keys from the specified directory.
func LoadKeys(options *Options) error {
	// Load the keys
	privKeyPath := path.Join(options.KeysDir, "privateKey")

	// read the private key from the file
	privKeyFile, err := os.Open(filepath.Clean(privKeyPath))
	if err != nil {
		return err
	}
	defer privKeyFile.Close()

	// base64 encoded private key
	privKey, err := io.ReadAll(privKeyFile)
	if err != nil {
		return err
	}

	base64PrivKey := base64.StdEncoding.EncodeToString(privKey)
	wgtypesKey, err := wgtypes.ParseKey(base64PrivKey)
	if err != nil {
		return err
	}

	options.PrivateKey = wgtypesKey
	return nil
}
