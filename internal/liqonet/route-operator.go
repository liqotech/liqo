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
package liqonetOperators

import (
	"context"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"net"
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
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL}
)

const (
	LiqonetPostroutingChain              = "LIQO-POSTROUTING"
	LiqonetPreroutingChain               = "LIQO-PREROUTING"
	LiqonetForwardingChain               = "LIQO-FORWARD"
	LiqonetInputChain                    = "LIQO-INPUT"
	LiqonetPostroutingClusterChainPrefix = "LIQO-PSTRT-CLS-"
	LiqonetPreroutingClusterChainPrefix  = "LIQO-PRRT-CLS-"
	LiqonetForwardingClusterChainPrefix  = "LIQO-FRWD-CLS-"
	LiqonetInputClusterChainPrefix       = "LIQO-INPT-CLS-"
	NatTable                             = "nat"
	FilterTable                          = "filter"
)

// RouteController reconciles a TunnelEndpoint object
type RouteController struct {
	client.Client
	Scheme         *runtime.Scheme
	clientset      kubernetes.Clientset
	Recorder       record.EventRecorder
	NodeName       string
	ClientSet      *kubernetes.Clientset
	RemoteVTEPs    []string
	IsGateway      bool
	VxlanNetwork   string
	GatewayVxlanIP string
	VxlanIfaceName string
	VxlanPort      int
	IPtables       liqonetOperator.IPTables
	NetLink        liqonetOperator.NetLink
	ClusterPodCIDR string
	Configured     chan bool //channel to comunicate when the podCIDR has been set
	IsConfigured   bool      //true when the operator is configured and ready to be started
	//here we save only the rules that reference the custom chains added by us
	//we need them at deletion time
	IPTablesRuleSpecsReferencingChains map[string]liqonetOperator.IPtableRule //using a map to avoid duplicate entries. the key is the rulespec
	//here we save the custom iptables chains, this chains are added at startup time so there should not be duplicates
	//but we use a map to avoid them in case the operator crashes and then is restarted by kubernetes
	IPTablesChains         map[string]liqonetOperator.IPTableChain
	RoutesPerRemoteCluster map[string]netlink.Route
	RetryTimeout           time.Duration
}

// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list

func (r *RouteController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	var tep netv1alpha1.TunnelEndpoint
	//name of our finalizer
	routeOperatorFinalizer := "routeOperator-" + r.NodeName + "-liqo.io"

	if err := r.Get(ctx, req.NamespacedName, &tep); err != nil {
		klog.Errorf("unable to fetch resource %s", req.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	clusterID := tep.Spec.ClusterID
	//we can process a tunnelendpoint resource only if the tunnel interface has been created and configured
	//this network interface is managed by the tunnel operator who sets this field when process the same tunnelendpoint
	//resource and creates the network interface.
	//if it has not been created yet than we return and wait for the resource to be processed by the tunnel operator.
	if tep.Status.TunnelIFaceName == "" {
		klog.Infof("%s -> the tunnel network interface is not ready", clusterID)
		return result, nil
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if tep.ObjectMeta.DeletionTimestamp.IsZero() {
		if !liqonetOperator.ContainsString(tep.ObjectMeta.Finalizers, routeOperatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			tep.ObjectMeta.Finalizers = append(tep.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				//while updating we check if the a resource version conflict happened
				//which means the version of the object we have is outdated.
				//a solution could be to return an error and requeue the object for later process
				//but if the object has been changed by another instance of the controller running in
				//another host it already has been put in the working queue so decide to forget the
				//current version and process the next item in the queue assured that we handle the object later
				if k8sApiErrors.IsConflict(err) {
					return result, nil
				}
				klog.Errorf("%s -> unable to add finalizers to resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
	} else {
		//the object is being deleted
		//if we encounter an error while removing iptables rules or the routes than we record an
		//event on the resource to notify the user
		//the finalizer is not removed
		if liqonetOperator.ContainsString(tep.Finalizers, routeOperatorFinalizer) {
			if err := r.removeIPTablesPerCluster(&tep); err != nil {
				klog.Errorf("%s -> unable to delete iptables rules for resource %s: %s", clusterID, req.String(), err)
				r.Recorder.Event(&tep, "Warning", "Delete", err.Error())
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
			//remove the finalizer from the list and update it.
			tep.Finalizers = liqonetOperator.RemoveString(tep.Finalizers, routeOperatorFinalizer)
			if err := r.Update(ctx, &tep); err != nil {
				if k8sApiErrors.IsConflict(err) {
					return result, nil
				}
				klog.Errorf("%s -> unable to remove finalizers from resource %s: %s", clusterID, req.String(), err)
				return result, err
			}
		}
		return result, nil
	}
	if err := r.ensureIPTablesRulesPerCluster(&tep); err != nil {
		klog.Errorf("%s -> unable to insert iptables rules for resource %s: %s", clusterID, req.String(), err)
		r.Recorder.Event(&tep, "Warning", "Processing", err.Error())
		return result, err
	} else {
		r.Recorder.Event(&tep, "Normal", "Processing", "iptables rules ensured")
	}
	if err := r.ensureRoutesPerCluster(&tep); err != nil {
		klog.Errorf("%s -> unable to add routes for resource %s: %s", clusterID, req.String(), err)
		r.Recorder.Event(&tep, "Warning", "Processing", err.Error())
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
	} else {
		r.Recorder.Event(&tep, "Normal", "Processing", "routes ensured")
	}
	return result, nil
}

func (r *RouteController) GetPodCIDRS(tep *netv1alpha1.TunnelEndpoint) (string, string) {
	var remotePodCIDR, localRemappedPodCIDR string
	if tep.Status.RemoteRemappedPodCIDR != "None" {
		remotePodCIDR = tep.Status.RemoteRemappedPodCIDR
	} else {
		remotePodCIDR = tep.Spec.PodCIDR
	}
	localRemappedPodCIDR = tep.Status.LocalRemappedPodCIDR
	return localRemappedPodCIDR, remotePodCIDR
}

func (r *RouteController) GetPostroutingRules(tep *netv1alpha1.TunnelEndpoint) ([]string, error) {
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := r.GetPodCIDRS(tep)
	if r.IsGateway {
		if localRemappedPodCIDR != defaultPodCIDRValue {
			//we get the first IP address from the podCIDR of the local cluster
			//in this case it is the podCIDR to which the local podCIDR has bee remapped by the remote peering cluster
			natIP, _, err := net.ParseCIDR(localRemappedPodCIDR)
			if err != nil {
				klog.Errorf("%s -> unable to get the IP from localPodCidr %s used to NAT the traffic from localhosts to remote hosts", clusterID, localRemappedPodCIDR)
				return nil, err
			}
			return []string{
				strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "NETMAP", "--to", localRemappedPodCIDR}, " "),
				strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "SNAT", "--to-source", natIP.String()}, " "),
			}, nil
		}
		//we get the first IP address from the podCIDR of the local cluster
		natIP, _, err := net.ParseCIDR(r.ClusterPodCIDR)
		if err != nil {
			klog.Errorf("%s -> unable to get the IP from localPodCidr %s used to NAT the traffic from localhosts to remote hosts", clusterID, tep.Spec.PodCIDR)
			return nil, err
		}
		return []string{
			strings.Join([]string{"!", "-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "SNAT", "--to-source", natIP.String()}, " "),
			strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}, " "),
		}, nil
	}
	return []string{
		strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}, " "),
	}, nil
}

func (r *RouteController) InsertRulesIfNotPresent(clusterID, table, chain string, rules []string) error {
	for _, rule := range rules {
		exists, err := r.IPtables.Exists(table, chain, strings.Split(rule, " ")...)
		if err != nil {
			klog.Errorf("%s -> unable to check if rule '%s' exists in chain %s in table %s", clusterID, rule, chain, table)
			return err
		}
		if !exists {
			if err := r.IPtables.AppendUnique(table, chain, strings.Split(rule, " ")...); err != nil {
				return err
			}
			klog.Infof("%s -> inserting rule '%s' in chain %s in table %s", clusterID, rule, chain, table)
		}
	}
	return nil
}

func (r *RouteController) UpdateRulesPerChain(clusterID, chain, table string, existingRules, newRules []string) error {
	//remove the outdated rules
	//if the chain has been newly created than the for loop will do nothing
	for _, existingRule := range existingRules {
		outdated := true
		for _, newRule := range newRules {
			if existingRule == newRule {
				outdated = false
			}
		}
		if outdated {
			if err := r.IPtables.Delete(table, chain, strings.Split(existingRule, " ")...); err != nil {
				return err
			}
			klog.Infof("%s -> removing outdated rule '%s' from chain %s in table %s", clusterID, existingRule, chain, table)
		}
	}
	if err := r.InsertRulesIfNotPresent(clusterID, table, chain, newRules); err != nil {
		return err
	}
	return nil
}

func (r *RouteController) ListRulesInChain(table, chain string) ([]string, error) {
	existingRules, err := r.IPtables.List(table, chain)
	if err != nil {
		return nil, err
	}
	rules := make([]string, 0)
	ruleToRemove := strings.Join([]string{"-N", chain}, " ")
	for _, rule := range existingRules {
		if rule != ruleToRemove {
			tmp := strings.Split(rule, " ")
			rules = append(rules, strings.Join(tmp[2:], " "))
		}
	}
	return rules, nil
}

func (r *RouteController) ensureIPTablesRulesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	if err := r.ensureChainRulespecs(tep); err != nil {
		return err
	}
	if err := r.ensurePostroutingRules(tep); err != nil {
		return err
	}
	if err := r.ensurePreroutingRules(tep); err != nil {
		return err
	}
	if err := r.ensureForwardRules(tep); err != nil {
		return err
	}
	if err := r.ensureInputRules(tep); err != nil {
		return err
	}
	return nil
}

func (r *RouteController) CreateIptablesChainIfNotExists(table string, newChain string) error {
	//get existing chains
	chains_list, err := r.IPtables.ListChains(table)
	if err != nil {
		return fmt.Errorf("imposible to retrieve chains in table -> %s : %v", table, err)
	}
	//if the chain exists do nothing
	for _, chain := range chains_list {
		if chain == newChain {
			return nil
		}
	}
	//if we come here the chain does not exist so we insert it
	err = r.IPtables.NewChain(table, newChain)
	if err != nil {
		return fmt.Errorf("unable to create %s chain in %s table: %v", newChain, table, err)
	}
	klog.Infof("created chain %s in table %s", newChain, table)
	return nil
}

func (r *RouteController) InsertIptablesRulespecIfNotExists(table string, chain string, ruleSpec []string) error {
	//get the list of rulespecs for the specified chain
	rulesList, err := r.IPtables.List(table, chain)
	if err != nil {
		return fmt.Errorf("unable to get the rules in %s chain in %s table : %v", chain, table, err)
	}
	//here we check if the rulespec exists and at the same time if it exists more then once
	numOccurrences := 0
	for _, rule := range rulesList {
		if strings.Contains(rule, strings.Join(ruleSpec, " ")) {
			numOccurrences++
		}
	}
	//if the occurrences if greater then one, remove the rulespec
	if numOccurrences > 1 {
		for i := 0; i < numOccurrences; i++ {
			if err = r.IPtables.Delete(table, chain, ruleSpec...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
			}
		}
		if err = r.IPtables.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
	} else if numOccurrences == 1 {
		//if the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(rulesList[0], strings.Join(ruleSpec, " ")) {
			return nil
		}
		if err = r.IPtables.Delete(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		if err = r.IPtables.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		return nil
	} else if numOccurrences == 0 {
		//if the occurrence is zero then insert the rule in first position
		if err = r.IPtables.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		klog.Infof("installed rulespec '%s' in chain %s of table %s", strings.Join(ruleSpec, " "), chain, table)
	}
	return nil
}

func (r *RouteController) removeIPTablesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	chains := r.GetChainRulespecs(tep)
	clusterID := tep.Spec.ClusterID
	for _, chain := range chains {
		//flush chain
		err := r.IPtables.ClearChain(chain.table, chain.chainName)
		if err != nil {
			klog.Errorf("%s -> unable to flush chain %s: %s", clusterID, chain.chainName, err)
			return err
		}
		existingRules, err := r.IPtables.List(chain.table, chain.chain)
		if err != nil {
			klog.Errorf("%s -> unable to list rules in chain %s from table %s: %s", clusterID, chain.chain, chain.table, err)
			return err
		}
		//remove references to chain
		for _, rule := range existingRules {
			if strings.Contains(rule, chain.chainName) {
				if err := r.IPtables.Delete(chain.table, chain.chain, strings.Split(rule, " ")[2:]...); err != nil {
					return err
				}
				klog.Infof("%s -> removing rule '%s' from chain %s in table %s", clusterID, rule, chain.chain, chain.table)
			}
		}
		//delete chains
		if err := r.IPtables.DeleteChain(chain.table, chain.chainName); err != nil {
			klog.Errorf("%s -> unable to remove chain %s from table %s: %s", clusterID, chain.chainName, chain.table, err)
			return err
		}
	}
	return nil
}

func (r *RouteController) GetChainRulespecs(tep *netv1alpha1.TunnelEndpoint) []struct {
	chainName string
	rulespec  string
	table     string
	chain     string
} {
	clusterID := tep.Spec.ClusterID
	localRemappedPodCIDR, remotePodCIDR := r.GetPodCIDRS(tep)
	postRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	forwardChain := strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	inputChain := strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	if localRemappedPodCIDR != defaultPodCIDRValue {
		return []struct {
			chainName string
			rulespec  string
			table     string
			chain     string
		}{
			{
				postRoutingChain,
				strings.Join([]string{"-d", remotePodCIDR, "-j", postRoutingChain}, " "),
				NatTable,
				LiqonetPostroutingChain,
			},
			{
				preRoutingChain,
				strings.Join([]string{"-d", localRemappedPodCIDR, "-j", preRoutingChain}, " "),
				NatTable,
				LiqonetPreroutingChain,
			},
			{
				forwardChain,
				strings.Join([]string{"-d", remotePodCIDR, "-j", forwardChain}, " "),
				FilterTable,
				LiqonetForwardingChain,
			},
			{
				inputChain,
				strings.Join([]string{"-d", remotePodCIDR, "-j", inputChain}, " "),
				FilterTable,
				LiqonetInputChain,
			},
		}
	}
	return []struct {
		chainName string
		rulespec  string
		table     string
		chain     string
	}{
		{
			postRoutingChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", postRoutingChain}, " "),
			NatTable,
			LiqonetPostroutingChain,
		},
		{
			forwardChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", forwardChain}, " "),
			FilterTable,
			LiqonetForwardingChain,
		},
		{
			inputChain,
			strings.Join([]string{"-d", remotePodCIDR, "-j", inputChain}, " "),
			FilterTable,
			LiqonetInputChain,
		},
	}

}

func (r *RouteController) ensureChainRulespecs(tep *netv1alpha1.TunnelEndpoint) error {
	chains := r.GetChainRulespecs(tep)
	clusterID := tep.Spec.ClusterID
	for _, chain := range chains {
		//create chain for the peering cluster if it does not exist
		err := r.CreateIptablesChainIfNotExists(chain.table, chain.chainName)
		if err != nil {
			klog.Errorf("%s -> unable to create chain %s: %s", clusterID, chain.chainName, err)
			return err
		}
		existingRules, err := r.IPtables.List(chain.table, chain.chain)
		if err != nil {
			klog.Errorf("%s -> unable to list rules in chain %s from table %s: %s", clusterID, chain.chain, chain.table, err)
			return err
		}
		for _, rule := range existingRules {
			if strings.Contains(rule, chain.chainName) {
				if !strings.Contains(rule, chain.rulespec) {
					if err := r.IPtables.Delete(chain.table, chain.chain, strings.Split(rule, " ")[2:]...); err != nil {
						return err
					}
					klog.Infof("%s -> removing outdated rule '%s' from chain %s in table %s", clusterID, rule, chain.chain, chain.table)
				}
			}
		}
		err = r.InsertRulesIfNotPresent(clusterID, chain.table, chain.chain, []string{chain.rulespec})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RouteController) ensurePostroutingRules(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	postRoutingChain := strings.Join([]string{LiqonetPostroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	rules, err := r.GetPostroutingRules(tep)
	if err != nil {
		return err
	}
	//list rules in the chain
	existingRules, err := r.ListRulesInChain(NatTable, postRoutingChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, postRoutingChain, NatTable, err)
		return err
	}
	return r.UpdateRulesPerChain(clusterID, postRoutingChain, NatTable, existingRules, rules)
}

func (r *RouteController) ensurePreroutingRules(tep *netv1alpha1.TunnelEndpoint) error {
	//if the node is not a gateway node then return
	if !r.IsGateway {
		return nil
	}
	//check if we need to NAT the incoming traffic from the peering cluster
	localRemappedPodCIDR, _ := r.GetPodCIDRS(tep)
	if localRemappedPodCIDR == defaultPodCIDRValue {
		return nil
	}
	clusterID := tep.Spec.ClusterID
	tunnelIFace := tep.Status.TunnelIFaceName
	preRoutingChain := strings.Join([]string{LiqonetPreroutingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")
	//list rules in the chain
	existingRules, err := r.ListRulesInChain(NatTable, preRoutingChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, preRoutingChain, NatTable, err)
		return err
	}
	rules := []string{
		strings.Join([]string{"-d", localRemappedPodCIDR, "-i", tunnelIFace, "-j", "NETMAP", "--to", r.ClusterPodCIDR}, " "),
	}
	return r.UpdateRulesPerChain(clusterID, preRoutingChain, NatTable, existingRules, rules)
}

func (r *RouteController) ensureForwardRules(tep *netv1alpha1.TunnelEndpoint) error {
	_, remotePodCIDR := r.GetPodCIDRS(tep)
	clusterID := tep.Spec.ClusterID
	forwardChain := strings.Join([]string{LiqonetForwardingClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")

	//list rules in the chain
	existingRules, err := r.ListRulesInChain(FilterTable, forwardChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, forwardChain, NatTable, err)
		return err
	}
	rules := []string{
		strings.Join([]string{"-d", remotePodCIDR, "-j", "ACCEPT"}, " "),
	}
	return r.UpdateRulesPerChain(clusterID, forwardChain, FilterTable, existingRules, rules)
}

func (r *RouteController) ensureInputRules(tep *netv1alpha1.TunnelEndpoint) error {
	_, remotePodCIDR := r.GetPodCIDRS(tep)
	clusterID := tep.Spec.ClusterID
	inputChain := strings.Join([]string{LiqonetInputClusterChainPrefix, strings.Split(clusterID, "-")[0]}, "")

	//list rules in the chain
	existingRules, err := r.ListRulesInChain(FilterTable, inputChain)
	if err != nil {
		klog.Errorf("%s -> unable to list rules for chain %s in table %s: %s", clusterID, inputChain, FilterTable, err)
		return err
	}
	rules := []string{
		strings.Join([]string{"-s", r.ClusterPodCIDR, "-d", remotePodCIDR, "-j", "ACCEPT"}, " "),
	}
	return r.UpdateRulesPerChain(clusterID, inputChain, FilterTable, existingRules, rules)
}

//this function is called at startup of the operator
//here we:
//create LIQONET-FORWARD in the filter table and insert it in the "FORWARD" chain
//create LIQONET-POSTROUTING in the nat table and insert it in the "POSTROUTING" chain
//create LIQONET-INPUT in the filter table and insert it in the input chain
//insert the rulespec which allows in input all the udp traffic incoming for the vxlan in the LIQONET-INPUT chain
func (r *RouteController) CreateAndEnsureIPTablesChains() error {
	var err error
	ipt := r.IPtables
	//creating LIQONET-POSTROUTING chain
	if err = r.CreateIptablesChainIfNotExists(NatTable, LiqonetPostroutingChain); err != nil {
		return err
	}
	r.IPTablesChains[LiqonetPostroutingChain] = liqonetOperator.IPTableChain{
		Table: NatTable,
		Name:  LiqonetPostroutingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-POSTROUTING chain
	forwardToLiqonetPostroutingRuleSpec := []string{"-j", LiqonetPostroutingChain}
	if err = r.InsertIptablesRulespecIfNotExists(NatTable, "POSTROUTING", forwardToLiqonetPostroutingRuleSpec); err != nil {
		return err
	}
	//add it to iptables rulespec if it does not exist in the map
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetPostroutingRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    NatTable,
		Chain:    "POSTROUTING",
		RuleSpec: forwardToLiqonetPostroutingRuleSpec,
	}
	//creating LIQONET-PREROUTING chain
	if err = r.CreateIptablesChainIfNotExists(NatTable, LiqonetPreroutingChain); err != nil {
		return err
	}
	r.IPTablesChains[LiqonetPreroutingChain] = liqonetOperator.IPTableChain{
		Table: NatTable,
		Name:  LiqonetPreroutingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-PREROUTING chain
	forwardToLiqonetPreroutingRuleSpec := []string{"-j", LiqonetPreroutingChain}
	if err = r.InsertIptablesRulespecIfNotExists(NatTable, "PREROUTING", forwardToLiqonetPreroutingRuleSpec); err != nil {
		return err
	}
	//add it to iptables rulespec if it does not exist in the map
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetPreroutingRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    NatTable,
		Chain:    "PREROUTING",
		RuleSpec: forwardToLiqonetPreroutingRuleSpec,
	}
	//creating LIQONET-FORWARD chain
	if err = r.CreateIptablesChainIfNotExists(FilterTable, LiqonetForwardingChain); err != nil {
		return err
	}
	r.IPTablesChains[LiqonetForwardingChain] = liqonetOperator.IPTableChain{
		Table: FilterTable,
		Name:  LiqonetForwardingChain,
	}
	//installing rulespec which forwards all traffic to LIQONET-FORWARD chain
	forwardToLiqonetForwardRuleSpec := []string{"-j", LiqonetForwardingChain}
	if err = r.InsertIptablesRulespecIfNotExists(FilterTable, "FORWARD", forwardToLiqonetForwardRuleSpec); err != nil {
		return err
	}
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetForwardRuleSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    FilterTable,
		Chain:    "FORWARD",
		RuleSpec: forwardToLiqonetForwardRuleSpec,
	}
	//creating LIQONET-INPUT chain
	if err = r.CreateIptablesChainIfNotExists(FilterTable, LiqonetInputChain); err != nil {
		return err
	}
	r.IPTablesChains[LiqonetInputChain] = liqonetOperator.IPTableChain{
		Table: FilterTable,
		Name:  LiqonetInputChain,
	}
	//installing rulespec which forwards all udp incoming traffic to LIQONET-INPUT chain
	forwardToLiqonetInputSpec := []string{"-p", "udp", "-m", "udp", "-j", LiqonetInputChain}
	if err = r.InsertIptablesRulespecIfNotExists(FilterTable, "INPUT", forwardToLiqonetInputSpec); err != nil {
		return err
	}
	r.IPTablesRuleSpecsReferencingChains[strings.Join(forwardToLiqonetInputSpec, " ")] = liqonetOperator.IPtableRule{
		Table:    FilterTable,
		Chain:    "INPUT",
		RuleSpec: forwardToLiqonetInputSpec,
	}
	//installing rulespec which allows udp traffic with destination port the VXLAN port
	//we put it here because this rulespec is independent from the remote cluster.
	//we don't save this rulespec it will be removed when the chains are flushed at exit time
	vxlanUdpRuleSpec := []string{"-p", "udp", "-m", "udp", "--dport", strconv.Itoa(r.VxlanPort), "-j", "ACCEPT"}
	exists, err := ipt.Exists(FilterTable, LiqonetInputChain, vxlanUdpRuleSpec...)
	if err != nil {
		return err
	}
	if !exists {
		if err = ipt.AppendUnique(FilterTable, LiqonetInputChain, vxlanUdpRuleSpec...); err != nil {
			return err
		}
		klog.Infof("installed rulespec '%s' in chain %s of table %s", strings.Join(vxlanUdpRuleSpec, " "), LiqonetInputChain, FilterTable)
	}
	return nil
}

//this function is called when the route-operator program is closed
//the errors are not checked because the function is called at exit time
//it cleans up all the possible resources
//a log message is emitted if in case of error
//only if the iptables binaries are missing an error is returned
func (r *RouteController) removeAllIPTablesChains(teps netv1alpha1.TunnelEndpointList) {
	var err error
	ipt := r.IPtables
	for i := range teps.Items {
		//the program is closing do not check the error but try to remove all the possible external resources and log the errors
		_ = r.removeIPTablesPerCluster(&teps.Items[i])
	}

	//first all the iptables chains are flushed
	for _, chain := range r.IPTablesChains {
		if err = ipt.ClearChain(chain.Table, chain.Name); err != nil {
			klog.Errorf("unable to flush chain %s in table %s", chain.Name, chain.Table)
		}
	}
	//second we delete the references to the chains
	for k, rulespec := range r.IPTablesRuleSpecsReferencingChains {
		if err = ipt.Delete(rulespec.Table, rulespec.Chain, rulespec.RuleSpec...); err != nil {
			e, ok := err.(*iptables.Error)
			if ok && e.IsNotExist() {
				delete(r.IPTablesRuleSpecsReferencingChains, k)
			} else if !ok {
				klog.Errorf("unable to delete rule %s in chain %s from table %s", strings.Join(rulespec.RuleSpec, ""), rulespec.Chain, rulespec.Table)
			}
		}
	}
	//then we delete the chains which now should be empty
	for _, chain := range r.IPTablesChains {
		if err = ipt.DeleteChain(chain.Table, chain.Name); err != nil {
			klog.Errorf("unable to delete chain %s from table %s", chain.Name, chain.Table)
		}
	}
}

func (r *RouteController) ensureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	clusterID := tep.Spec.ClusterID
	_, remotePodCIDR := r.GetPodCIDRS(tep)
	if r.IsGateway {
		existing, ok := r.RoutesPerRemoteCluster[clusterID]
		if ok {
			//check if the network parameters are the same and if we need to remove the old route and add the new one
			if existing.LinkIndex == tep.Status.TunnelIFaceIndex && existing.Gw.String() == "" && existing.Dst.String() == remotePodCIDR {
				return nil
			}
			//remove the old route
			err := r.NetLink.DelRoute(existing)
			if err != nil {
				klog.Errorf("%s -> unable to remove old route '%s': %s", clusterID, remotePodCIDR, err)
				return err
			}
		}
		route, err := r.NetLink.AddRoute(remotePodCIDR, "", tep.Status.TunnelIFaceName, false)
		if err != nil {
			klog.Errorf("%s -> unable to insert route for subnet %s on device %s: %s", clusterID, remotePodCIDR, tep.Status.TunnelIFaceName, err)
			return err
		}
		r.RoutesPerRemoteCluster[clusterID] = route
	} else {
		existing, ok := r.RoutesPerRemoteCluster[clusterID]
		//check if the network parameters are the same and if we need to remove the old route and add the new one
		if ok {
			if existing.Gw.String() == r.GatewayVxlanIP && existing.Dst.String() == remotePodCIDR {
				return nil
			}
			//remove the old route
			err := r.NetLink.DelRoute(existing)
			if err != nil {
				klog.Errorf("%s -> unable to remove old route '%s': %s", clusterID, remotePodCIDR, err)
				return err
			}
		}

		route, err := r.NetLink.AddRoute(remotePodCIDR, r.GatewayVxlanIP, r.VxlanIfaceName, false)
		if err != nil {
			klog.Errorf("%s -> unable to insert route for subnet %s on device %s with gatewayIP %s: %s", clusterID, remotePodCIDR, r.VxlanIfaceName, r.GatewayVxlanIP, err)
			return err
		}
		r.RoutesPerRemoteCluster[clusterID] = route
	}
	return nil
}

//used to remove the routes when a tunnelEndpoint CR is removed
func (r *RouteController) removeRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) error {
	route, ok := r.RoutesPerRemoteCluster[tep.Spec.ClusterID]
	if ok {
		err := r.NetLink.DelRoute(route)
		if err != nil {
			return err
		}
		klog.Infof("%s -> route '%s' removed", tep.Spec.ClusterID, route)
	}
	return nil
}

func (r *RouteController) removeAllRoutes() {
	//the errors are not checked because the function is called at exit time
	//it cleans up all the possible resources
	//a log message is emitted if in case of error
	//get all teps and for each of them remove the routes
	for clusterID, route := range r.RoutesPerRemoteCluster {
		err := r.NetLink.DelRoute(route)
		if err != nil {
			klog.Errorf("%s -> unable to remove route '%s': %s", clusterID, route, err)
		}
		klog.Infof("%s -> route '%s' removed", route, clusterID)
	}
}

//this function deletes the vxlan interface in host where the route operator is running
func (r *RouteController) deleteVxlanIFace() {
	//first get the iface index
	iface, err := netlink.LinkByName(r.VxlanIfaceName)
	if err != nil {
		klog.Errorf("unable to get vxlan interface %s: %s", r.VxlanIfaceName, err)
		return
	}
	err = liqonetOperator.DeleteIFaceByIndex(iface.Attrs().Index)
	if err != nil {
		klog.Errorf("unable to remove vxlan interface %s: %s", r.VxlanIfaceName, err)
	}
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT. A stop channel is returned
// which is closed on one of these signals.
func (r *RouteController) SetupSignalHandlerForRouteOperator(quit, waitCleanUp chan struct{}) (stopCh <-chan struct{}) {
	klog.Info("Entering signal handler")
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func(r *RouteController) {
		sig := <-c
		klog.Infof("received signal %s: cleaning up before exiting", sig.String())
		close(quit)
		//get all teps and for each of them remove the iptables rules
		teps := netv1alpha1.TunnelEndpointList{}
		err := r.List(context.Background(), &teps)
		if err != nil {
			klog.Errorf("unable to list all tunnelEndpoint resources: %s", err)
		}
		close(stop)
		r.removeAllIPTablesChains(teps)
		r.removeAllRoutes()
		r.deleteVxlanIFace()
		close(waitCleanUp)
	}(r)
	return stop
}

func (r *RouteController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			//finalizers are used to check if a resource is being deleted, and perform there the needed actions
			//we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).WithEventFilter(resourceToBeProccesedPredicate).
		For(&netv1alpha1.TunnelEndpoint{}).
		Complete(r)
}
