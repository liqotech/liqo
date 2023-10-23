// Copyright 2019-2023 The Liqo Authors
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

package ipset

import (
	"k8s.io/utils/exec"

	ipset "github.com/liqotech/liqo/pkg/liqonet/ipset/kubernetes"
)

// IPSHandler is a handler exposing functions to use the ipset utility.
type IPSHandler struct {
	ips ipset.Interface
}

// NewIPSHandler create a new IPSHandler.
func NewIPSHandler() IPSHandler {
	ipset.ValidIPSetTypes = append(ipset.ValidIPSetTypes, ipset.HashIP)
	return IPSHandler{
		ips: ipset.New(exec.New()),
	}
}

// CreateSet creates a new set.
func (h *IPSHandler) CreateSet(name, comment string) (*ipset.IPSet, error) {
	ipSet := newSet(name, comment)
	if err := h.ips.CreateSet(ipSet, true); err != nil {
		return nil, err
	}
	return ipSet, nil
}

// DestroySet deletes a named set.
func (h *IPSHandler) DestroySet(name string) error {
	return h.ips.DestroySet(name)
}

// FlushSet deletes all entries from a named set.
func (h *IPSHandler) FlushSet(name string) error {
	return h.ips.FlushSet(name)
}

// ListSets list all set names.
func (h *IPSHandler) ListSets() ([]string, error) {
	sets, err := h.ips.ListSets()
	if err != nil {
		return nil, err
	}
	return sets, nil
}

// ListEntries lists all the entries from a named set.
func (h *IPSHandler) ListEntries(set string) ([]string, error) {
	entries, err := h.ips.ListEntries(set)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// AddEntry adds a new entry to the named set.
func (h *IPSHandler) AddEntry(ip string, set *ipset.IPSet) error {
	err := h.ips.AddEntry(ip, set, true)
	return err
}

func newSet(name, comment string) *ipset.IPSet {
	return &ipset.IPSet{
		Name:    name,
		SetType: ipset.HashIP,
		Comment: comment,
	}
}
