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

package route

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/runtime"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	// RTTablesDir contains path to the directory with ID to routing tables IDs mapping in Linux.
	RTTablesDir = "/etc/iproute2"
	// RTTablesFilename contains path to the file with ID to routing tables IDs mapping in Linux.
	RTTablesFilename = RTTablesDir + "/rt_tables"
)

// EnsureTablePresence ensures the presence of the given table.
func EnsureTablePresence(routeconfiguration *networkingv1beta1.RouteConfiguration, tableID uint32) error {
	exists, err := ExistsTableID(tableID)
	if err != nil {
		return err
	}
	if !exists {
		if err := AddTableID(tableID, routeconfiguration.Spec.Table.Name); err != nil {
			return err
		}
	}
	return nil
}

// EnsureTableAbsence ensures the absence of the given table.
func EnsureTableAbsence(tableID uint32) error {
	exists, err := ExistsTableID(tableID)
	if err != nil {
		return err
	}
	if exists {
		if err := DeleteTableID(tableID); err != nil {
			return err
		}
	}
	return nil
}

// GetTableID returns the table ID associated with the given route.
func GetTableID(tableName string) (uint32, error) {
	if tableName == "" {
		return 0, fmt.Errorf("table name is empty")
	}
	return generateTableID(tableName), nil
}

// ExistsTableID checks if the given table ID is already present in the rt_tables file.
func ExistsTableID(tableID uint32) (exists bool, err error) {
	if tableID == 0 {
		return false, fmt.Errorf("table ID is empty")
	}

	file, err := os.OpenFile(RTTablesFilename, os.O_RDONLY, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer func() {
		runtime.Must(file.Close())
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		entry := scanner.Text()
		if strings.Contains(entry, fmt.Sprintf("%d", tableID)) {
			return true, nil
		}
	}
	return false, nil
}

// AddTableID adds the given table ID to the rt_tables file.
func AddTableID(tableID uint32, tableName string) error {
	newEntry := forgeTableEntry(tableID, tableName)

	// ensure the directory existence
	if err := os.MkdirAll(RTTablesDir, 0o700); err != nil {
		return err
	}

	file, err := os.OpenFile(RTTablesFilename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		runtime.Must(file.Close())
	}()

	if _, err := fmt.Fprintf(file, "%s\n", newEntry); err != nil {
		return err
	}
	return nil
}

// DeleteTableID deletes the given table ID from the rt_tables file.
func DeleteTableID(tableID uint32) error {
	lines, err := filterDeletedLines(tableID)
	if err != nil {
		return err
	}

	// ensure the directory existence
	if err := os.MkdirAll(RTTablesDir, 0o700); err != nil {
		return err
	}

	file, err := os.OpenFile(RTTablesFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		runtime.Must(file.Close())
	}()
	for _, line := range lines {
		if _, err := fmt.Fprintf(file, "%s\n", line); err != nil {
			return err
		}
	}
	return nil
}

func filterDeletedLines(tableID uint32) ([]string, error) {
	file, err := os.OpenFile(RTTablesFilename, os.O_RDONLY, 0o600)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}
	defer func() {
		runtime.Must(file.Close())
	}()
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		entry := scanner.Text()
		if !strings.Contains(entry, fmt.Sprintf("%d", tableID)) {
			lines = append(lines, entry)
		}
	}
	return lines, nil
}

func generateTableID(name string) uint32 {
	hash := sha256.Sum256([]byte(name))
	id := hash[0:4]
	// the first bit of the most significant byte must be 0. https://serverfault.com/questions/315705/how-many-custom-route-tables-can-i-have-on-linux
	id[3] >>= 1
	// IDs in the range 0 <= ID <= 255 are used by the operating system
	// make sure we won't use this range
	id[1] |= 1
	return uint32(hash[3])<<24 | uint32(hash[2])<<16 | uint32(hash[1])<<8 | uint32(hash[0])
}

func forgeTableEntry(tableID uint32, tableName string) string {
	return fmt.Sprintf("%s\t%s", strconv.FormatUint(uint64(tableID), 10), tableName)
}
