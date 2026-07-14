// Copyright 2019-2026 The Liqo Authors
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
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ti-mo/conntrack"
	"k8s.io/klog/v2"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

// conntrackClient abstracts the subset of conntrack.Conn used by the reconciler.
type conntrackClient interface {
	Dump(*conntrack.DumpOptions) ([]conntrack.Flow, error)
	Delete(*conntrack.Flow) error
	Close() error
}

// conntrackConn wraps a *conntrack.Conn to satisfy conntrackClient.
type conntrackConn struct {
	*conntrack.Conn
}

func (c *conntrackConn) Dump(opts *conntrack.DumpOptions) ([]conntrack.Flow, error) {
	return c.Conn.Dump(opts)
}

func (c *conntrackConn) Delete(f *conntrack.Flow) error {
	return c.Conn.Delete(*f)
}

// newConntrackConn opens a new conntrack netlink connection.
func newConntrackConn() (conntrackClient, error) {
	c, err := conntrack.Dial(nil)
	if err != nil {
		return nil, err
	}
	return &conntrackConn{c}, nil
}

// flushConntrackForNotrackRules removes existing conntrack entries that match
// any notrack rule in the firewallconfiguration. Existing flows are not affected
// by newly-added notrack expressions, so they need to be evicted explicitly.
func (r *FirewallConfigurationReconciler) flushConntrackForNotrackRules(ctx context.Context, fwcfg *networkingv1beta1.FirewallConfiguration) error {
	if r.ConntrackClient == nil {
		return nil
	}

	notrackRules := notrackRules(&fwcfg.Spec.Table)
	if len(notrackRules) == 0 {
		return nil
	}

	klog.V(4).Infof("Removing conntrack flows matching notrack rules in firewallconfiguration %s/%s",
		fwcfg.Namespace, fwcfg.Name)

	flows, err := r.ConntrackClient.Dump(nil)
	if err != nil {
		return fmt.Errorf("dumping conntrack flows: %w", err)
	}

	deleted := 0
	for i := range flows {
		if ctx.Err() != nil {
			return fmt.Errorf("context canceled while deleting conntrack flows: %w", ctx.Err())
		}
		if !flowMatchesNotrackRules(&flows[i], notrackRules) {
			continue
		}
		if err := r.ConntrackClient.Delete(&flows[i]); err != nil {
			return fmt.Errorf("deleting conntrack flow %v: %w", flows[i], err)
		}
		deleted++
	}

	if deleted > 0 {
		klog.Infof("Deleted %d conntrack flows matching notrack rules in firewallconfiguration %s/%s",
			deleted, fwcfg.Namespace, fwcfg.Name)
	}

	return nil
}

// notrackRules returns all filter rules with ActionNotrack found in the table.
func notrackRules(table *firewallapi.Table) []firewallapi.FilterRule {
	if table == nil {
		return nil
	}
	var rules []firewallapi.FilterRule
	for i := range table.Chains {
		for j := range table.Chains[i].Rules.FilterRules {
			if table.Chains[i].Rules.FilterRules[j].Action == firewallapi.ActionNotrack {
				rules = append(rules, table.Chains[i].Rules.FilterRules[j])
			}
		}
	}
	return rules
}

// flowMatchesNotrackRules returns true if the flow matches at least one of the
// provided notrack rules in either direction.
func flowMatchesNotrackRules(flow *conntrack.Flow, rules []firewallapi.FilterRule) bool {
	for i := range rules {
		if ruleMatchesFlow(&rules[i], flow) {
			return true
		}
	}
	return false
}

// ruleMatchesFlow returns true if the flow matches the rule in either the
// original or the reply direction.
func ruleMatchesFlow(rule *firewallapi.FilterRule, flow *conntrack.Flow) bool {
	return tupleMatchesRule(&flow.TupleOrig, rule) || tupleMatchesRule(&flow.TupleReply, rule)
}

// tupleMatchesRule checks whether a conntrack tuple satisfies all the match
// criteria of the given rule.
func tupleMatchesRule(tuple *conntrack.Tuple, rule *firewallapi.FilterRule) bool {
	for i := range rule.Match {
		if !matchSatisfiesRule(&rule.Match[i], tuple) {
			return false
		}
	}
	return true
}

// matchSatisfiesRule evaluates a single rule Match against a conntrack tuple.
// The Op field is respected (eq/neq). Dev matches are ignored because they are
// not part of a conntrack flow.
func matchSatisfiesRule(match *firewallapi.Match, tuple *conntrack.Tuple) bool {
	matched := true

	if match.IP != nil {
		matched = matched && matchIP(tuple, match.IP)
	}
	if match.Port != nil {
		matched = matched && matchPort(tuple, match.Port)
	}
	if match.Proto != nil {
		matched = matched && matchProto(tuple, match.Proto)
	}
	// Dev matches are ignored: conntrack flows do not carry ingress/egress
	// device information, so they cannot change the result.

	if match.Op == firewallapi.MatchOperationNeq {
		return !matched
	}
	return matched
}

func matchIP(tuple *conntrack.Tuple, ip *firewallapi.MatchIP) bool {
	prefix, err := netip.ParsePrefix(ip.Value)
	if err != nil {
		addr, err := netip.ParseAddr(ip.Value)
		if err != nil {
			return false
		}
		prefix = netip.PrefixFrom(addr, addr.BitLen())
	}

	switch ip.Position {
	case firewallapi.MatchPositionSrc:
		return prefix.Contains(tuple.IP.SourceAddress)
	case firewallapi.MatchPositionDst:
		return prefix.Contains(tuple.IP.DestinationAddress)
	default:
		return false
	}
}

func matchPort(tuple *conntrack.Tuple, port *firewallapi.MatchPort) bool {
	start, end, ok := parsePortRange(port.Value)
	if !ok {
		return false
	}

	switch port.Position {
	case firewallapi.MatchPositionSrc:
		return portInRange(tuple.Proto.SourcePort, start, end)
	case firewallapi.MatchPositionDst:
		return portInRange(tuple.Proto.DestinationPort, start, end)
	default:
		return false
	}
}

func matchProto(tuple *conntrack.Tuple, proto *firewallapi.MatchProto) bool {
	switch proto.Value {
	case firewallapi.L4ProtoTCP:
		return tuple.Proto.Protocol == 6
	case firewallapi.L4ProtoUDP:
		return tuple.Proto.Protocol == 17
	default:
		return false
	}
}

// parsePortRange parses a port or a port range ("3000-4000"). If the value is
// not valid, ok is false.
func parsePortRange(value string) (start, end uint16, ok bool) {
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		s, err := strconv.ParseUint(parts[0], 10, 16)
		if err != nil {
			return 0, 0, false
		}
		e, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return 0, 0, false
		}
		if s > e {
			return 0, 0, false
		}
		return uint16(s), uint16(e), true
	}

	s, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, 0, false
	}
	return uint16(s), uint16(s), true
}

func portInRange(port, start, end uint16) bool {
	return port >= start && port <= end
}
