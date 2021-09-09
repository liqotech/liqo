// Copyright 2019-2021 The Liqo Authors
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

package ipam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	goipam "github.com/metal-stack/go-ipam"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	ipamNamePrefix              = "ipamstorage-"
	clusterSubnetUpdate         = "clusterSubnets"
	poolsUpdate                 = "pools"
	prefixesUpdate              = "prefixes"
	externalCIDRUpdate          = "externalCIDR"
	endpointMappingsUpdate      = "endpointMappings"
	podCIDRUpdate               = "podCIDR"
	serviceCIDRUpdate           = "serviceCIDR"
	natMappingsConfiguredUpdate = "natMappingsConfigured"
)

// IpamStorage is the interface to be implemented to enforce persistency in IPAM.
type IpamStorage interface {
	updateClusterSubnets(clusterSubnet map[string]netv1alpha1.Subnets) error
	updatePools(pools []string) error
	updateExternalCIDR(externalCIDR string) error
	updateEndpointMappings(endpoints map[string]netv1alpha1.EndpointMapping) error
	updatePodCIDR(podCIDR string) error
	updateServiceCIDR(serviceCIDR string) error
	updateNatMappingsConfigured(natMappingsConfigured map[string]netv1alpha1.ConfiguredCluster) error
	getClusterSubnets() (map[string]netv1alpha1.Subnets, error)
	getPools() ([]string, error)
	getExternalCIDR() (string, error)
	getEndpointMappings() (map[string]netv1alpha1.EndpointMapping, error)
	getPodCIDR() (string, error)
	getServiceCIDR() (string, error)
	getNatMappingsConfigured() (map[string]netv1alpha1.ConfiguredCluster, error)
	goipam.Storage
}

// IPAMStorage is an implementation of IpamStorage that takes advantage of the CRD IpamStorage.
type IPAMStorage struct {
	dynClient    dynamic.Interface
	resourceName string
}

// NewIPAMStorage inits the storage of the IPAM module,
// retrieving an existing ipamStorage resource or creating a new one.
func NewIPAMStorage(dynClient dynamic.Interface) (*IPAMStorage, error) {
	klog.Infof("Init IPAM storage..")
	ipamStorage := &IPAMStorage{}
	ipamStorage.dynClient = dynClient
	klog.Infof("Looking for Ipam resource..")
	ipam, err := ipamStorage.getConfig()
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if errors.IsNotFound(err) {
		klog.Infof("IPAM resource has not been found, creating a new one..")
		ipam = &netv1alpha1.IpamStorage{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "net.liqo.io/v1alpha1",
				Kind:       "IpamStorage",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ipamNamePrefix,
				Labels:       map[string]string{consts.IpamStorageResourceLabelKey: consts.IpamStorageResourceLabelValue},
			},
			Spec: netv1alpha1.IpamSpec{
				Prefixes:              make(map[string][]byte),
				Pools:                 make([]string, 0),
				ClusterSubnets:        make(map[string]netv1alpha1.Subnets),
				EndpointMappings:      make(map[string]netv1alpha1.EndpointMapping),
				NatMappingsConfigured: make(map[string]netv1alpha1.ConfiguredCluster),
			},
		}
		unstructuredIpam, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ipam)
		if err != nil {
			klog.Errorf("cannot map ipam resource to unstructured resource: %s", err.Error())
			return nil, err
		}
		up, err := ipamStorage.dynClient.
			Resource(netv1alpha1.IpamGroupResource).
			Create(context.Background(), &unstructured.Unstructured{Object: unstructuredIpam}, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("cannot create ipam resource: %s", err.Error())
			return nil, err
		}
		ipamStorage.resourceName = up.GetName()
		klog.Infof("Resource %s of type %s successfully created", ipamStorage.resourceName, netv1alpha1.GroupVersion)
	} else {
		ipamStorage.resourceName = ipam.Name
		klog.Infof("Resource %s of type %s has been found", ipamStorage.resourceName, netv1alpha1.IpamGroupResource)
	}
	klog.Infof("Ipam storage successfully configured")
	return ipamStorage, nil
}

// CreatePrefix creates a new Prefix in ipamStorage resource.
func (ipamStorage *IPAMStorage) CreatePrefix(prefix goipam.Prefix) (goipam.Prefix, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return goipam.Prefix{}, err
	}
	if _, ok := ipam.Spec.Prefixes[prefix.Cidr]; ok {
		return goipam.Prefix{}, fmt.Errorf("prefix already created:%v", prefix)
	}
	gob, err := prefix.GobEncode()
	ipam.Spec.Prefixes[prefix.Cidr] = gob
	if err != nil {
		return goipam.Prefix{}, fmt.Errorf("failed to encode prefix %s: %w", prefix.Cidr, err)
	}
	if err = ipamStorage.updatePrefixes(ipam.Spec.Prefixes); err != nil {
		klog.Errorf("cannot update ipam resource:%s", err.Error())
		return goipam.Prefix{}, err
	}
	return prefix, err
}

// ReadPrefix retrieves a specific Prefix from ipamStorage resource.
func (ipamStorage *IPAMStorage) ReadPrefix(prefix string) (goipam.Prefix, error) {
	var p goipam.Prefix
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return goipam.Prefix{}, err
	}
	if _, ok := ipam.Spec.Prefixes[prefix]; !ok {
		return goipam.Prefix{}, fmt.Errorf("prefix %s not found", prefix)
	}
	err = p.GobDecode(ipam.Spec.Prefixes[prefix])
	if err != nil {
		return goipam.Prefix{}, err
	}
	return p, nil
}

// ReadAllPrefixes retrieves all prefixes from ipamStorage resource.
func (ipamStorage *IPAMStorage) ReadAllPrefixes() ([]goipam.Prefix, error) {
	list := make([]goipam.Prefix, 0)
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return nil, err
	}
	for _, value := range ipam.Spec.Prefixes {
		var p goipam.Prefix
		err = p.GobDecode(value)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, err
}

// ReadAllPrefixCidrs retrieves all prefix CIDR from ipamStorage resource.
func (ipamStorage *IPAMStorage) ReadAllPrefixCidrs() ([]string, error) {
	list := make([]string, 0)
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return nil, err
	}
	for _, value := range ipam.Spec.Prefixes {
		var p goipam.Prefix
		err = p.GobDecode(value)
		if err != nil {
			return nil, err
		}
		list = append(list, p.Cidr)
	}
	return list, err
}

// UpdatePrefix updates a Prefix in ipamStorage resource.
func (ipamStorage *IPAMStorage) UpdatePrefix(prefix goipam.Prefix) (goipam.Prefix, error) {
	if prefix.Cidr == "" {
		return goipam.Prefix{}, fmt.Errorf("prefix not present:%v", prefix)
	}
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return goipam.Prefix{}, err
	}
	if _, ok := ipam.Spec.Prefixes[prefix.Cidr]; !ok {
		return goipam.Prefix{}, fmt.Errorf("prefix %s not found", prefix.Cidr)
	}
	gob, err := prefix.GobEncode()
	ipam.Spec.Prefixes[prefix.Cidr] = gob
	if err != nil {
		return goipam.Prefix{}, fmt.Errorf("cannot update prefix %s: %w", prefix.Cidr, err)
	}
	if err = ipamStorage.updatePrefixes(ipam.Spec.Prefixes); err != nil {
		klog.Errorf("cannot update ipam resource:%s", err.Error())
		return goipam.Prefix{}, err
	}
	return prefix, nil
}

// DeletePrefix deletes a Prefix from ipamStorage resource.
func (ipamStorage *IPAMStorage) DeletePrefix(prefix goipam.Prefix) (goipam.Prefix, error) {
	if prefix.Cidr == "" {
		return goipam.Prefix{}, fmt.Errorf("prefix not present:%v", prefix)
	}
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return goipam.Prefix{}, err
	}
	if _, ok := ipam.Spec.Prefixes[prefix.Cidr]; !ok {
		return goipam.Prefix{}, fmt.Errorf("prefix %s not found", prefix.Cidr)
	}
	delete(ipam.Spec.Prefixes, prefix.Cidr)
	if err = ipamStorage.updatePrefixes(ipam.Spec.Prefixes); err != nil {
		klog.Errorf("cannot update ipam resource:%s", err.Error())
		return goipam.Prefix{}, err
	}
	return prefix, err
}

func (ipamStorage *IPAMStorage) updateClusterSubnets(clusterSubnets map[string]netv1alpha1.Subnets) error {
	return ipamStorage.updateConfig(clusterSubnetUpdate, clusterSubnets)
}
func (ipamStorage *IPAMStorage) updatePools(pools []string) error {
	return ipamStorage.updateConfig(poolsUpdate, pools)
}
func (ipamStorage *IPAMStorage) updatePrefixes(prefixes map[string][]byte) error {
	return ipamStorage.updateConfig(prefixesUpdate, prefixes)
}
func (ipamStorage *IPAMStorage) updateExternalCIDR(externalCIDR string) error {
	return ipamStorage.updateConfig(externalCIDRUpdate, externalCIDR)
}
func (ipamStorage *IPAMStorage) updateEndpointMappings(endpoints map[string]netv1alpha1.EndpointMapping) error {
	return ipamStorage.updateConfig(endpointMappingsUpdate, endpoints)
}
func (ipamStorage *IPAMStorage) updatePodCIDR(podCIDR string) error {
	return ipamStorage.updateConfig(podCIDRUpdate, podCIDR)
}
func (ipamStorage *IPAMStorage) updateServiceCIDR(serviceCIDR string) error {
	return ipamStorage.updateConfig(serviceCIDRUpdate, serviceCIDR)
}

func (ipamStorage *IPAMStorage) updateNatMappingsConfigured(natMappingsConfigured map[string]netv1alpha1.ConfiguredCluster) error {
	return ipamStorage.updateConfig(natMappingsConfiguredUpdate, natMappingsConfigured)
}

func (ipamStorage *IPAMStorage) updateConfig(updateType string, data interface{}) error {
	jsonData, err := json.Marshal(data)

	if err != nil {
		klog.Errorf("cannot marshal object:%s", err.Error())
	}

	var b bytes.Buffer
	patch := fmt.Sprintf(
		`[{"op": "replace", "path": "/spec/%s", "value": `,
		updateType)
	b.WriteString(patch)
	b.Write(jsonData)
	b.WriteString("}]")

	_, err = ipamStorage.dynClient.Resource(netv1alpha1.IpamGroupResource).Patch(context.Background(),
		ipamStorage.resourceName,
		types.JSONPatchType,
		b.Bytes(),
		metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return nil
	}
	return nil
}

func (ipamStorage *IPAMStorage) getPools() ([]string, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return []string{}, err
	}
	return ipam.Spec.Pools, nil
}

func (ipamStorage *IPAMStorage) getClusterSubnets() (map[string]netv1alpha1.Subnets, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return map[string]netv1alpha1.Subnets{}, err
	}
	return ipam.Spec.ClusterSubnets, nil
}

func (ipamStorage *IPAMStorage) getExternalCIDR() (string, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return "", err
	}
	return ipam.Spec.ExternalCIDR, nil
}
func (ipamStorage *IPAMStorage) getEndpointMappings() (map[string]netv1alpha1.EndpointMapping, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return nil, err
	}
	return ipam.Spec.EndpointMappings, nil
}
func (ipamStorage *IPAMStorage) getPodCIDR() (string, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return "", err
	}
	return ipam.Spec.PodCIDR, nil
}
func (ipamStorage *IPAMStorage) getServiceCIDR() (string, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return "", err
	}
	return ipam.Spec.ServiceCIDR, nil
}

func (ipamStorage *IPAMStorage) getNatMappingsConfigured() (map[string]netv1alpha1.ConfiguredCluster, error) {
	ipam, err := ipamStorage.getConfig()
	if err != nil {
		return nil, err
	}
	return ipam.Spec.NatMappingsConfigured, nil
}

func (ipamStorage *IPAMStorage) getConfig() (*netv1alpha1.IpamStorage, error) {
	res := &netv1alpha1.IpamStorage{}
	list, err := ipamStorage.dynClient.
		Resource(netv1alpha1.IpamGroupResource).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", consts.IpamStorageResourceLabelKey, consts.IpamStorageResourceLabelValue),
		})
	if err != nil {
		klog.Errorf(err.Error())
		return nil, fmt.Errorf("unable to get configuration: %w", err)
	}
	if len(list.Items) != 1 {
		if len(list.Items) != 0 {
			return nil, fmt.Errorf("multiple resources of type %s found", netv1alpha1.IpamGroupResource)
		}
		return nil, errors.NewNotFound(netv1alpha1.IpamGroupResource.GroupResource(), "")
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, res)
	if err != nil {
		return nil, fmt.Errorf("cannot map unstructured resource to ipam storage resource:%w", err)
	}
	// The following check allows user to define its own ipam resource.
	// If properly labeled, the module IPAM will take its configuration from the new resource
	if res.Name != ipamStorage.resourceName && ipamStorage.resourceName != "" {
		klog.Infof("IPAM configuration resource is %s now", res.Name)
		ipamStorage.resourceName = res.Name
	}
	return res, nil
}
