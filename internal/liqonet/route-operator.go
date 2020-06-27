/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controllers

import (
	"context"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
	"github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	liqonetOperator "github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	liqonetPostroutingChain = "LIQONET-POSTROUTING"
	liqonetPreroutingChain  = "LIQONET-PREROUTING"
	liqonetForwardingChain  = "LIQONET-FORWARD"
	liqonetInputChain       = "LIQONET-INPUT"
	natTable                = "nat"
	filterTable             = "filter"
	shutdownSignals         = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL}
)

// RouteController reconciles a TunnelEndpoint object
type RouteController struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	clientset      kubernetes.Clientset
	RouteOperator  bool
	NodeName       string
	ClientSet      *kubernetes.Clientset
	RemoteVTEPs    []string
	IsGateway      bool
	VxlanNetwork   string
	GatewayVxlanIP string
	VxlanIfaceName string
	VxlanPort      int
	ClusterPodCIDR string
	//here we save only the rules that reference the custom chains added by us
	//we need them at deletion time
	IPTablesRuleSpecsReferencingChains map[string]liqonetOperator.IPtableRule //using a map to avoid duplicate entries. the key is the rulespec
	//here we save the custom iptables chains, this chains are added at startup time so there should not be duplicates
	//but we use a map to avoid them in case the operator crashes and then is restarted by kubernetes
	IPTablesChains map[string]liqonetOperator.IPTableChain
	//for each cluster identified by clusterID we save all the rulespecs needed to ensure communication with its pods
	IPtablesRuleSpecsPerRemoteCluster map[string][]liqonetOperator.IPtableRule
	//here we save routes associated to each remote cluster
	RoutesPerRemoteCluster map[string][]netlink.Route
	RetryTimeout  time.Duration
}

// +kubebuilder:rbac:groups=liqonet.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=liqonet.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list

func (r *RouteController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("endpoint", req.NamespacedName)
	var endpoint v1.TunnelEndpoint
	//name of our finalizer
	routeOperatorFinalizer := "routeOperator-" + r.NodeName + "-Finalizer.liqonet.liqo.io"

	if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		r.Log.Error(err, "unable to fetch endpoint")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if endpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		if !liqonetOperator.ContainsString(endpoint.ObjectMeta.Finalizers, routeOperatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			endpoint.ObjectMeta.Finalizers = append(endpoint.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				//while updating we check if the a resource version conflict happened
				//which means the version of the object we have is outdated.
				//a solution could be to return an error and requeue the object for later process
				//but if the object has been changed by another instance of the controller running in
				//another host it already has been put in the working queue so decide to forget the
				//current version and process the next item in the queue assured that we handle the object later
				if k8sApiErrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				r.Log.Error(err, "unable to update endpoint")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
		}
	} else {
		//the object is being deleted
		if liqonetOperator.ContainsString(endpoint.Finalizers, routeOperatorFinalizer) {
			if err := r.deleteIPTablesRulespecForRemoteCluster(&endpoint); err != nil {
				r.Log.Error(err, "error while deleting rulespec from iptables")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
			if err := r.deleteRoutesPerCluster(&endpoint); err != nil {
				r.Log.Error(err, "error while deleting routes")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
			//remove the finalizer from the list and update it.
			endpoint.Finalizers = liqonetOperator.RemoveString(endpoint.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				if k8sApiErrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				r.Log.Error(err, "unable to update")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
		}
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}
	if !r.alreadyProcessedByRouteOperator(endpoint.GetObjectMeta()) {
		if err := r.createAndInsertIPTablesChains(); err != nil {
			r.Log.Error(err, "unable to create iptables chains")
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
		if err := r.addIPTablesRulespecForRemoteCluster(&endpoint); err != nil {
			log.Error(err, "unable to insert ruleSpec")
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
		if err := r.InsertRoutesPerCluster(&endpoint); err != nil {
			log.Error(err, "unable to insert routes")
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
		endpoint.ObjectMeta.SetLabels(liqonetOperator.SetLabelHandler(liqonetOperator.RouteOpLabelKey+"-"+r.NodeName, "ready", endpoint.ObjectMeta.GetLabels()))
		err := r.Client.Update(ctx, &endpoint)
		for k8sApiErrors.IsConflict(err) {
			log.Info("a resource version conflict arose while updating", "resource", req.NamespacedName)
			if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
				r.Log.Error(err, "unable to fetch endpoint")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, client.IgnoreNotFound(err)
			}
			endpoint.ObjectMeta.SetLabels(liqonetOperator.SetLabelHandler(liqonetOperator.RouteOpLabelKey+"-"+r.NodeName, "ready", endpoint.ObjectMeta.GetLabels()))
			err = r.Client.Update(ctx, &endpoint)
		}
		if err != nil {
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
	}
	return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
}

//this function is called at startup of the operator
//here we:
//create LIQONET-FORWARD in the filter table and insert it in the "FORWARD" chain
//create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain
//create LIQONET-INPUT in the filter table and insert it in the input chain
//insert the rulespec which allows in input all the udp traffic incoming for the vxlan in the LIQONET-INPUT chain
func (r *RouteController) createAndInsertIPTablesChains() error {
	ipt, err := iptables.New()
	log := r.Log.WithName("iptables")
	if err != nil {
		return fmt.Errorf("unable to initialize iptables: %v. check if the ipatable are present in the system", err)
	}

	//creating LIQONET-POSTROUTING chain
	if err = liqonetOperator.CreateIptablesChainsIfNotExist(ipt, natTable, liqonetPostroutingChain); err != nil {
		return err
	} else {
		log.Info("created", "chain", liqonetPostroutingChain, "in table", natTable)
	}
	r.IPTablesChains[liqonetPostroutingChain] = liqonetOperator.IPTableChain{
		Table: natTable,
		Name:  liqonetPostroutingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-POSTROUTING chain
	forwardToLiqonetPostroutingRuleSpec := []string{"-j", liqonetPostroutingChain}
	if err = liqonetOperator.InsertIptablesRulespecIfNotExists(ipt, natTable, "POSTROUTING", forwardToLiqonetPostroutingRuleSpec); err != nil {
		return err
	} else {
		log.Info("installed", "rulespec", strings.Join(forwardToLiqonetPostroutingRuleSpec, " "), "belonging to chain POSTROUTING in table", natTable)
	}
	//add it to iptables rulespec if it does not exist in the map
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetPostroutingRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    natTable,
		Chain:    "POSTROUTING",
		RuleSpec: forwardToLiqonetPostroutingRuleSpec,
	}
	//creating LIQONET-PREROUTING chain
	if err = liqonetOperator.CreateIptablesChainsIfNotExist(ipt, natTable, liqonetPreroutingChain); err != nil {
		return err
	} else {
		log.Info("created", "chain", liqonetPreroutingChain, "in table", natTable)
	}
	r.IPTablesChains[liqonetPostroutingChain] = liqonetOperator.IPTableChain{
		Table: natTable,
		Name:  liqonetPreroutingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-PREROUTING chain
	forwardToLiqonetPreroutingRuleSpec := []string{"-j", liqonetPreroutingChain}
	if err = liqonetOperator.InsertIptablesRulespecIfNotExists(ipt, natTable, "PREROUTING", forwardToLiqonetPreroutingRuleSpec); err != nil {
		return err
	} else {
		log.Info("installed", "rulespec", strings.Join(forwardToLiqonetPreroutingRuleSpec, " "), "belonging to chain POSTROUTING in table", natTable)
	}
	//add it to iptables rulespec if it does not exist in the map
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetPreroutingRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    natTable,
		Chain:    "PREROUTING",
		RuleSpec: forwardToLiqonetPreroutingRuleSpec,
	}
	//creating LIQONET-FORWARD chain
	if err = liqonetOperator.CreateIptablesChainsIfNotExist(ipt, filterTable, liqonetForwardingChain); err != nil {
		return err
	} else {
		log.Info("created", "chain", liqonetForwardingChain, "in table", filterTable)
	}
	r.IPTablesChains[liqonetForwardingChain] = liqonetOperator.IPTableChain{
		Table: filterTable,
		Name:  liqonetForwardingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-FORWARD chain
	forwardToLiqonetForwardRuleSpec := []string{"-j", liqonetForwardingChain}
	if err = liqonetOperator.InsertIptablesRulespecIfNotExists(ipt, filterTable, "FORWARD", forwardToLiqonetForwardRuleSpec); err != nil {
		return err
	} else {
		log.Info("installed", "rulespec", strings.Join(forwardToLiqonetForwardRuleSpec, " "), "belonging to chain FORWARD in table", filterTable)
	}
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetForwardRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    filterTable,
		Chain:    "FORWARD",
		RuleSpec: forwardToLiqonetForwardRuleSpec,
	}
	//creating LIQONET-INPUT chain
	if err = liqonetOperator.CreateIptablesChainsIfNotExist(ipt, filterTable, liqonetInputChain); err != nil {
		return err
	} else {
		log.Info("created", "chain", liqonetInputChain, "in table", filterTable)
	}
	r.IPTablesChains[liqonetInputChain] = liqonetOperator.IPTableChain{
		Table: filterTable,
		Name:  liqonetInputChain,
	}
	//installing rulespec which forwards all udp incoming traffic to LIQONET-INPUT chain
	forwardToLiqonetInputSpec := []string{"-p", "udp", "-m", "udp", "-j", liqonetInputChain}
	if err = liqonetOperator.InsertIptablesRulespecIfNotExists(ipt, filterTable, "INPUT", forwardToLiqonetInputSpec); err != nil {
		return err
	} else {
		log.Info("installed", "rulespec", strings.Join(forwardToLiqonetInputSpec, " "), "belonging to chain POSTROUTING in table", filterTable)
	}
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetInputSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    filterTable,
		Chain:    "INPUT",
		RuleSpec: forwardToLiqonetInputSpec,
	}
	//installing rulespec which allows udp traffic with destination port the VXLAN port
	//we put it here because this rulespec is independent from the remote cluster.
	//we don't save this rulespec it will be removed when the chains are flushed at exit time
	//TODO: do we need to move this one elsewhere? maybe in a dedicate function called at startup by the route operator?
	vxlanUdpRuleSpec := []string{"-p", "udp", "-m", "udp", "--dport", strconv.Itoa(r.VxlanPort), "-j", "ACCEPT"}
	if err = ipt.AppendUnique(filterTable, liqonetInputChain, vxlanUdpRuleSpec...); err != nil {
		return fmt.Errorf("unable to insert rulespec \"%s\" in %s table and %s chain: %v", vxlanUdpRuleSpec, filterTable, liqonetInputChain, err)
	} else {
		log.Info("installed", "rulespec", strings.Join(vxlanUdpRuleSpec, " "), "belonging to chain", liqonetInputChain, "in table", filterTable)
	}
	return nil
}

func (r *RouteController) addIPTablesRulespecForRemoteCluster(endpoint *v1.TunnelEndpoint) error {
	var remotePodCIDR string
	clusterID := endpoint.Spec.ClusterID
	log := r.Log.WithName("iptables")
	if endpoint.Status.RemoteRemappedPodCIDR != "None" && endpoint.Status.RemoteRemappedPodCIDR != "" {
		remotePodCIDR = endpoint.Status.RemoteRemappedPodCIDR
		log.Info("nat enabled", "pod cidr of cluster", clusterID, "remapped from", endpoint.Spec.PodCIDR, "to", endpoint.Status.RemoteRemappedPodCIDR)
	} else {
		remotePodCIDR = endpoint.Spec.PodCIDR
		log.Info("nat disabled", "using original pod cidr", endpoint.Spec.PodCIDR, "for cluster", clusterID)
	}
	var ruleSpecs []liqonetOperator.IPtableRule
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("unable to initialize iptables: %v. check if the ipatable are present in the system", err)
	}

	//do not nat the traffic directed to the remote pods
	ruleSpec := []string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}
	if err = ipt.AppendUnique(natTable, liqonetPostroutingChain, ruleSpec...); err != nil {
		return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, natTable, liqonetPostroutingChain, err)
	} else {
		log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetPostroutingChain, "in table", filterTable)
	}
	ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
		Table:    natTable,
		Chain:    liqonetPostroutingChain,
		RuleSpec: ruleSpec,
	})
	r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
	//enable forwarding for all the traffic directed to the remote pods
	ruleSpec = []string{"-d", remotePodCIDR, "-j", "ACCEPT"}
	if err = ipt.AppendUnique(filterTable, liqonetForwardingChain, ruleSpec...); err != nil {
		return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, filterTable, liqonetForwardingChain, err)
	} else {
		log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetForwardingChain, "in table", filterTable)
	}
	ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
		Table:    filterTable,
		Chain:    liqonetForwardingChain,
		RuleSpec: ruleSpec,
	})
	r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
	//this rules are needed in an environment where strictly policies are applied for the input chain
	ruleSpec = []string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}
	if err = ipt.AppendUnique(filterTable, liqonetInputChain, ruleSpec...); err != nil {
		return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, filterTable, liqonetInputChain, err)
	} else {
		log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetInputChain, "in table", filterTable)
	}
	ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
		Table:    filterTable,
		Chain:    liqonetInputChain,
		RuleSpec: ruleSpec,
	})
	r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
	if r.IsGateway {
		//all the traffic coming from the hosts and directed to the remote pods is natted using the LocalTunnelPrivateIP
		//hosts use the ip of the vxlan interface as source ip when communicating with remote pods
		//this is done on the gateway node only
		ruleSpec = []string{"-s", r.VxlanNetwork, "-d", remotePodCIDR, "-j", "MASQUERADE"}
		if err = ipt.AppendUnique(natTable, liqonetPostroutingChain, ruleSpec...); err != nil {
			return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, natTable, liqonetPostroutingChain, err)
		} else {
			log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetPostroutingChain, "in table", natTable)
		}
		ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
			Table:    natTable,
			Chain:    liqonetPostroutingChain,
			RuleSpec: ruleSpec,
		})
		r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
		//if we have been remapped by the remote cluster then insert the iptables rule to masquerade the source ip
		if endpoint.Status.LocalRemappedPodCIDR != "None" {
			ruleSpec = []string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "NETMAP", "--to", endpoint.Status.LocalRemappedPodCIDR}
			if err = liqonetOperator.InsertIptablesRulespecIfNotExists(ipt, natTable, liqonetPostroutingChain, ruleSpec); err != nil {
				return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, natTable, liqonetPostroutingChain, err)
			} else {
				log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetPostroutingChain, "in table", natTable)
			}
			ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
				Table:    natTable,
				Chain:    liqonetPostroutingChain,
				RuleSpec: ruleSpec,
			})
			r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
			//translate all the traffic coming to the local cluster in to the right podcidr because it has been remapped by the remote cluster
			ruleSpec = []string{"-d", endpoint.Status.LocalRemappedPodCIDR, "-i", endpoint.Status.TunnelIFaceName, "-j", "NETMAP", "--to", r.ClusterPodCIDR}
			if err = ipt.AppendUnique(natTable, liqonetPreroutingChain, ruleSpec...); err != nil {
				return fmt.Errorf("unable to insert iptable rule \"%s\" in %s table, %s chain: %v", ruleSpec, natTable, liqonetPreroutingChain, err)
			} else {
				log.Info("installed", "rulespec", strings.Join(ruleSpec, " "), "belonging to chain", liqonetPreroutingChain, "in table", natTable)
			}
			ruleSpecs = append(ruleSpecs, liqonetOperator.IPtableRule{
				Table:    natTable,
				Chain:    liqonetPreroutingChain,
				RuleSpec: ruleSpec,
			})
			r.IPtablesRuleSpecsPerRemoteCluster[clusterID] = ruleSpecs
		}
	}
	return nil
}

//remove all the rules added by addIPTablesRulespecForRemoteCluster function
func (r *RouteController) deleteIPTablesRulespecForRemoteCluster(endpoint *v1.TunnelEndpoint) error {
	clusterID := endpoint.Spec.ClusterID
	log := r.Log.WithName("iptables")
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("unable to initialize iptables: %v. check if the ipatable are present in the system", err)
	}
	//retrive the iptables rules for the remote cluster
	rules, ok := r.IPtablesRuleSpecsPerRemoteCluster[endpoint.Spec.ClusterID]
	if ok {
		for _, rule := range rules {
			if err = ipt.Delete(rule.Table, rule.Chain, rule.RuleSpec...); err != nil {
				// if the rule that we are trying to delete does not exist then we are fine and go on
				e, ok := err.(*iptables.Error)
				if ok && e.IsNotExist() {
					continue
				} else if !ok {
					return fmt.Errorf("unable to delete iptable rule \"%s\" in %s table, %s chain: %v", strings.Join(rule.RuleSpec, " "), rule.Table, rule.Chain, err)
				}
			} else {
				log.Info("removing", "rulespec", strings.Join(rule.RuleSpec, " "), "belonging to chain", rule.Chain, "in table", rule.Table)
			}
		}
	}
	//after all the iptables rules have been removed then we delete them from the map
	//this is safe to do even if the key does not exist
	delete(r.IPtablesRuleSpecsPerRemoteCluster, clusterID)
	return nil
}

//this function is called when the route-operator program is closed
//the errors are not checked because the function is called at exit time
//it cleans up all the possible resources
//a log message is emitted if in case of error
//only if the iptables binaries are missing an error is returned
func (r *RouteController) DeleteAllIPTablesChains() {
	logger := r.Log.WithName("DeleteAllIPTablesChains")
	ipt, err := iptables.New()
	if err != nil {
		logger.Error(err, "unable to initialize iptables.check if the ipatable are present in the system")
	}
	//first all the iptables chains are flushed
	for _, chain := range r.IPTablesChains {
		if err = ipt.ClearChain(chain.Table, chain.Name); err != nil {
			e, ok := err.(*iptables.Error)
			if ok && e.IsNotExist() {
				continue
			} else if !ok {
				logger.Error(err, "unable to clear: ", "chain", chain.Name, "in table", chain.Table)
			}

		}
	}
	//second we delete the references to the chains
	for _, rulespec := range r.IPTablesRuleSpecsReferencingChains {
		if err = ipt.Delete(rulespec.Table, rulespec.Chain, rulespec.RuleSpec...); err != nil {
			e, ok := err.(*iptables.Error)
			if ok && e.IsNotExist() {
				continue
			} else if !ok {
				logger.Error(err, "unable to delete: ", "rule", strings.Join(rulespec.RuleSpec, ""), "in chain", rulespec.Chain, "in table", rulespec.Table)
			}
		}
	}
	//then we delete the chains which now should be empty
	for _, chain := range r.IPTablesChains {
		if err = ipt.DeleteChain(chain.Table, chain.Name); err != nil {
			e, ok := err.(*iptables.Error)
			if ok && e.IsNotExist() {
				continue
			} else if !ok {
				logger.Error(err, "unable to delete ", "chain", chain.Name, "in table", chain.Table)
			}
		}
	}
}

func (r *RouteController) InsertRoutesPerCluster(endpoint *v1.TunnelEndpoint) error {
	clusterID := endpoint.Spec.ClusterID
	log := r.Log.WithName("route")
	remoteTunnelPrivateIPNet := endpoint.Status.RemoteTunnelPrivateIP + "/32"
	var remotePodCIDR string
	localTunnelPrivateIP := endpoint.Status.LocalTunnelPrivateIP
	if endpoint.Status.RemoteRemappedPodCIDR != "None" && endpoint.Status.RemoteRemappedPodCIDR != "" {
		remotePodCIDR = endpoint.Status.RemoteRemappedPodCIDR
		log.Info("installing routes for", "cluster", clusterID, "with remapped pod cidr", remotePodCIDR)
	} else {
		remotePodCIDR = endpoint.Spec.PodCIDR
		log.Info("installing routes for", "cluster", clusterID, "with original pod cidr", remotePodCIDR)
	}
	var routes []netlink.Route
	if r.IsGateway {
		route, err := liqonetOperator.AddRoute(remoteTunnelPrivateIPNet, localTunnelPrivateIP, endpoint.Status.TunnelIFaceName, false)
		if err != nil {
			return err
		} else {
			log.Info("installing", "route", route.String())
		}
		routes = append(routes, route)
		route, err = liqonetOperator.AddRoute(remotePodCIDR, endpoint.Status.RemoteTunnelPrivateIP, endpoint.Status.TunnelIFaceName, true)
		if err != nil {
			return err
		} else {
			log.Info("installing", "route", route.String())
		}
		routes = append(routes, route)
		r.RoutesPerRemoteCluster[endpoint.Spec.ClusterID] = routes
	} else {
		route, err := liqonetOperator.AddRoute(remotePodCIDR, r.GatewayVxlanIP, r.VxlanIfaceName, false)
		if err != nil {
			return err
		} else {
			log.Info("installing", "route", route.String())
		}
		routes = append(routes, route)
		route, err = liqonetOperator.AddRoute(remoteTunnelPrivateIPNet, r.GatewayVxlanIP, r.VxlanIfaceName, false)
		if err != nil {
			return err
		} else {
			log.Info("installing", "route", route.String())
		}
		routes = append(routes, route)
		r.RoutesPerRemoteCluster[endpoint.Spec.ClusterID] = routes
	}
	return nil
}

//used to remove the routes when a tunnelEndpoint CR is removed
func (r *RouteController) deleteRoutesPerCluster(endpoint *v1.TunnelEndpoint) error {
	clusterID := endpoint.Spec.ClusterID
	log := r.Log.WithName("route")
	log.Info("removing all routes for", "cluster", clusterID)
	for _, route := range r.RoutesPerRemoteCluster[endpoint.Spec.ClusterID] {
		err := liqonetOperator.DelRoute(route)
		if err != nil {
			return err
		} else {
			log.Info("deleting", "route", route.String())
		}
	}
	//after all the routes have been removed then we delete them from the map
	//this is safe to do even if the key does not exist
	delete(r.RoutesPerRemoteCluster, clusterID)
	return nil
}

func (r *RouteController) deleteAllRoutes() {
	logger := r.Log.WithName("DeleteAllRoutes")
	//the errors are not checked because the function is called at exit time
	//it cleans up all the possible resources
	//a log message is emitted if in case of error
	for _, cluster := range r.RoutesPerRemoteCluster {
		for _, route := range cluster {
			if err := liqonetOperator.DelRoute(route); err != nil {
				logger.Error(err, "an error occurred while deleting", "route", route.String())
			}
		}
	}
}

//this function deletes the vxlan interface in host where the route operator is running
func (r *RouteController) deleteVxlanIFace() {
	logger := r.Log.WithName("DeleteVxlanIFace")
	//first get the iface index
	iface, err := netlink.LinkByName(r.VxlanIfaceName)
	if err != nil {
		logger.Error(err, "an error occurred while removing vxlan interface", "ifaceName", r.VxlanIfaceName)
	}
	err = liqonetOperator.DeleteIFaceByIndex(iface.Attrs().Index)
	if err != nil {
		logger.Error(err, "an error occurred while removing vxlan interface", "ifaceName", r.VxlanIfaceName)
	}
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT. A stop channel is returned
// which is closed on one of these signals.
func (r *RouteController) SetupSignalHandlerForRouteOperator() (stopCh <-chan struct{}) {
	logger := r.Log.WithValues("Route Operator Signal Handler", r.NodeName)
	fmt.Printf("Entering signal handler")
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func(r *RouteController) {
		sig := <-c
		logger.Info("received ", "signal", sig.String())
		r.DeleteAllIPTablesChains()
		r.deleteAllRoutes()
		r.deleteVxlanIFace()
		<-c
		close(stop)
	}(r)
	return stop
}

//checks if all the values need to install routes have ben set in the CR status
func (r *RouteController) ValidateCRAndReturn(endpoint *v1.TunnelEndpoint) bool {
	isReady := true
	//check if the tunnel interface is installed but only if the route operator is running on the gatewayhost
	if endpoint.Status.TunnelIFaceIndex != 0 && r.IsGateway {
		_, err := netlink.LinkByIndex(endpoint.Status.TunnelIFaceIndex)
		if err != nil {
			isReady = false
		}
	}
	if endpoint.Status.RemoteTunnelPrivateIP == "" || endpoint.Status.RemoteTunnelPublicIP == "" {
		isReady = false
	}
	if endpoint.Status.LocalTunnelPrivateIP == "" || endpoint.Status.LocalTunnelPublicIP == "" {
		isReady = false
	}
	return isReady
}
func (r *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.ToBeProcessedByRouteOperator(e.Meta)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			//finalizers are used to check if a resource is being deleted, and perform there the needed actions
			//we don't want to reconcile on the delete of a resource.
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.ToBeProcessedByRouteOperator(e.MetaNew)
		},
	}
	return ctrl.NewControllerManagedBy(mgr).WithEventFilter(resourceToBeProccesedPredicate).
		For(&v1.TunnelEndpoint{}).
		Complete(r)
}

func (r *RouteController) ToBeProcessedByRouteOperator(meta metav1.Object) bool {
	labels := meta.GetLabels()
	if labels == nil {
		return false
	}
	_, processedByTunOP := labels[liqonetOperator.TunOpLabelKey]
	if processedByTunOP {
		return true
	} else {
		return false
	}
}

func (r *RouteController) alreadyProcessedByRouteOperator(meta metav1.Object) bool {
	labels := meta.GetLabels()
	if labels == nil {
		return true
	}
	_, processedByRouOp := labels[liqonetOperator.RouteOpLabelKey+"-"+r.NodeName]
	if processedByRouOp {
		return true
	} else {
		return false
	}
}
