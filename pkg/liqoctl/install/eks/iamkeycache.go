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

package eks

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type iamUserCredential struct {
	AccessKeyID     string `yaml:"accessKeyID"`
	SecretAccessKey string `yaml:"secretAccessKey"`
}

type iamUserCredentialCache map[string]iamUserCredential

const (
	liqoIamCredentialsFile = "iam-credentials.yaml"
	liqoDirName            = ".liqo"
)

var (
	liqoDirPath string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	liqoDirPath = filepath.Join(homeDir, liqoDirName)
}

func storeIamAccessKey(iamUserName, accessKeyID, secretAccessKey string) error {
	cache, err := readCache()
	if err != nil {
		return err
	}

	cache[iamUserName] = iamUserCredential{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}

	data, err := yaml.Marshal(cache)
	if err != nil {
		return err
	}

	fileName := filepath.Join(liqoDirPath, liqoIamCredentialsFile)

	return os.WriteFile(fileName, data, 0o600)
}

func retrieveIamAccessKey(iamUserName string) (accessKeyID, secretAccessKey string, err error) {
	cache, err := readCache()
	if err != nil {
		return "", "", err
	}

	key, ok := cache[iamUserName]
	if !ok {
		return "", "", nil
	}

	return key.AccessKeyID, key.SecretAccessKey, nil
}

func readCache() (iamUserCredentialCache, error) {
	// ensure the directory existence
	err := os.MkdirAll(liqoDirPath, 0o700)
	if err != nil {
		return nil, err
	}

	fileName := filepath.Join(liqoDirPath, liqoIamCredentialsFile)
	if _, err := os.Stat(fileName); err != nil {
		return iamUserCredentialCache{}, nil
	}

	data, err := os.ReadFile(filepath.Clean(fileName))
	if err != nil {
		return nil, err
	}

	var cache iamUserCredentialCache
	if err = yaml.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return cache, nil
}
