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

package firewall

import (
	"fmt"

	"github.com/google/nftables"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/firewall/utils"
)

// cleanSets removes the sets that are no longer used in the firewall configuration and updates the existing ones if their elements differ from the wanted elements.
func cleanSets(nftconn *nftables.Conn, table *firewallapi.Table) error {
	// Find the table by name and family
	nftTables, err := nftconn.ListTablesOfFamily(getTableFamily(*table.Family))
	if err != nil {
		return err
	}

	var nftTable *nftables.Table
	for _, t := range nftTables {
		if t.Name == *table.Name {
			nftTable = t
			break
		}
	}

	if nftTable == nil {
		// Table does not exist, nothing to clean
		return nil
	}

	// Get all existing sets in the table
	nftSets, err := nftconn.GetSets(nftTable)
	if err != nil {
		return err
	}

	// Remove sets that are no longer used.
	if err := removeOutdatedSets(nftconn, nftSets, table); err != nil {
		return err
	}

	// Update existing sets if their elements differ from the wanted elements.
	if err := updateSetElements(nftconn, nftSets, table); err != nil {
		return err
	}

	return nil
}

// removeOutdatedSets removes the sets that are no longer used in the firewall configuration.
func removeOutdatedSets(nftconn *nftables.Conn, nftSets []*nftables.Set, table *firewallapi.Table) error {
	// Delete sets that are not used anymore in the table
	usedSetNames := make(map[string]interface{})
	for _, set := range table.Sets {
		usedSetNames[set.Name] = nil
	}

	for _, nftSet := range nftSets {
		if _, used := usedSetNames[nftSet.Name]; !used {
			// Set is not used anymore, delete it
			nftconn.DelSet(nftSet)
		}
	}

	return nil
}

// updateSetElements updates the elements of an existing set, if they differ from the wanted elements.
func updateSetElements(nftconn *nftables.Conn, nftSets []*nftables.Set, table *firewallapi.Table) error {
	for _, set := range table.Sets {
		// Find the corresponding nftables.Set
		var nftSet *nftables.Set
		for _, ns := range nftSets {
			if ns.Name == set.Name {
				nftSet = ns
				break
			}
		}

		if nftSet == nil {
			// Set does not exist, skip
			continue
		}

		// Get existing elements of the set
		existingElements, err := nftconn.GetSetElements(nftSet)
		if err != nil {
			return err
		}

		// Get wanted elements of the set
		wantedElements, err := genSetElements(&set)
		if err != nil {
			return err
		}

		// Check if the set is outdated
		if isSetOutdated(existingElements, wantedElements) {
			// Remove the existing elements and add the wanted ones
			if err := nftconn.SetDeleteElements(nftSet, existingElements); err != nil {
				return err
			}
			if err := nftconn.SetAddElements(nftSet, wantedElements); err != nil {
				return err
			}
		}
	}

	return nil
}

// isSetOutdated checks if the existing set elements differ from the wanted set elements.
func isSetOutdated(existingElements []nftables.SetElement, wantedElements []nftables.SetElement) bool {
	// If the lengths differ, the set is outdated.
	if len(existingElements) != len(wantedElements) {
		return true
	}

	// Build a map of existing elements for quick lookup.
	existingElementsMap := make(map[string]string)
	for _, element := range existingElements {
		existingElementsMap[string(element.Key)] = string(element.Val)
	}

	// Check if any wanted element is missing or differs in value.
	for _, wantedElement := range wantedElements {
		val, exists := existingElementsMap[string(wantedElement.Key)]
		if !exists || val != string(wantedElement.Val) {
			return true
		}
	}

	return false
}

// addSets adds the sets that are missing in the nftables configuration.
func addSets(nftconn *nftables.Conn, sets []firewallapi.Set, nftTable *nftables.Table) error {
	nftSets, err := nftconn.GetSets(nftTable)
	if err != nil {
		return err
	}
	existingSetNames := make(map[string]struct{})
	for _, nftSet := range nftSets {
		existingSetNames[nftSet.Name] = struct{}{}
	}

	for _, set := range sets {
		if _, exists := existingSetNames[set.Name]; !exists {
			_, err := addSet(nftconn, nftTable, &set)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// addSet adds a new set to the nftables configuration.
func addSet(nftconn *nftables.Conn, table *nftables.Table, set *firewallapi.Set) (*nftables.Set, error) {
	dataType, err := getSetDataType(set.DataType)
	if err != nil {
		return nil, err
	}

	keyType, err := getSetDataType(&set.KeyType)
	if err != nil {
		return nil, err
	}

	nftSet := &nftables.Set{
		Table:    table,
		Name:     set.Name,
		KeyType:  keyType,
		DataType: dataType,
	}

	setData, err := genSetElements(set)
	if err != nil {
		return nil, err
	}

	err = nftconn.AddSet(nftSet, setData)
	if err != nil {
		return nil, err
	}

	return nftSet, nil
}

func genSetElements(set *firewallapi.Set) ([]nftables.SetElement, error) {
	setData := make([]nftables.SetElement, len(set.Elements))
	for i, element := range set.Elements {
		data, err := utils.ConvertSetData(element.Data, set.DataType)
		if err != nil {
			return nil, fmt.Errorf("unable to convert set element data: %v", err)
		}

		key, err := utils.ConvertSetData(&element.Key, &set.KeyType)
		if err != nil {
			return nil, fmt.Errorf("unable to convert set element key: %v", err)
		}

		setData[i] = nftables.SetElement{
			Key: key,
			Val: data,
		}
	}

	return setData, nil
}

func getSetDataType(dataType *firewallapi.SetDataType) (nftables.SetDatatype, error) {
	if dataType == nil {
		return nftables.SetDatatype{}, nil
	}

	switch *dataType {
	case firewallapi.SetDataTypeIPAddr:
		return nftables.TypeIPAddr, nil
	default:
		return nftables.SetDatatype{}, fmt.Errorf("unsupported set data type: %s", *dataType)
	}
}
