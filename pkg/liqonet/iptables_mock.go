package liqonet

import (
	"strings"
)

// MockIPTables implementation of the IPTables interface used for test purposes.
type MockIPTables struct {
	Rules  []IPTableRule
	Chains []IPTableChain
}

// Exists checks if a given rulespec exists.
func (m *MockIPTables) Exists(table, chain string, rulespec ...string) (bool, error) {
	for _, rule := range m.Rules {
		if rule.Table == table && rule.Chain == chain && strings.Join(rule.RuleSpec, " ") == strings.Join(rulespec, " ") {
			return true, nil
		}
	}
	return false, nil
}

// AppendUnique appends a rule only if it does not exist.
func (m *MockIPTables) AppendUnique(table, chain string, rulespec ...string) error {
	r := IPTableRule{
		Table:    table,
		Chain:    chain,
		RuleSpec: rulespec,
	}
	if m.containsRule(r) {
		return nil
	}
	m.Rules = append(m.Rules, r)
	return nil
}

// ListChains lists all the chains for a given table.
func (m *MockIPTables) ListChains(table string) ([]string, error) {
	var chains []string
	for _, chain := range m.Chains {
		if chain.Table == table {
			chains = append(chains, chain.Name)
		}
	}
	return chains, nil
}

// NewChain creates a new chain in ghe given table.
func (m *MockIPTables) NewChain(table, chain string) error {
	m.Chains = append(m.Chains, IPTableChain{
		Table: table,
		Name:  chain,
	})
	return nil
}

func (m *MockIPTables) ruleIndex(table, chain string, rulespec []string) int {
	for i, rule := range m.Rules {
		if rule.Table == table && rule.Chain == chain && strings.Join(rule.RuleSpec, " ") == strings.Join(rulespec, " ") {
			return i
		}
	}
	return -1
}

func (m *MockIPTables) containsRule(rule IPTableRule) bool {
	for _, r := range m.Rules {
		if r.Table == rule.Table && r.Chain == rule.Chain && strings.Join(r.RuleSpec, " ") == strings.Join(rule.RuleSpec, " ") {
			return true
		}
	}
	return false
}

// Delete deletes the given rulespec.
func (m *MockIPTables) Delete(table, chain string, rulespec ...string) error {
	var ruleIndex = m.ruleIndex(table, chain, rulespec)
	if ruleIndex != -1 {
		m.Rules = append(m.Rules[:ruleIndex], m.Rules[ruleIndex+1:]...)
	}
	return nil
}

func (m *MockIPTables) prependRule(x []IPTableRule, y IPTableRule) []IPTableRule {
	x = append(x, IPTableRule{})
	copy(x[1:], x)
	x[0] = y
	return x
}

// Insert inserts a new rulespec
// this mock function prepends even if the index is different than 1.
func (m *MockIPTables) Insert(table, chain string, pos int, rulespec ...string) error {
	m.Rules = m.prependRule(m.Rules, IPTableRule{
		Table:    table,
		Chain:    chain,
		RuleSpec: rulespec,
	})
	return nil
}

// List lists all the existing rulespecs.
func (m *MockIPTables) List(table, chain string) ([]string, error) {
	var rules []string
	for _, rule := range m.Rules {
		if rule.Table == table {
			rules = append(rules, strings.Join(rule.RuleSpec, " "))
		}
	}
	return rules, nil
}

// ClearChain removes all the rulespecs for a given chain.
func (m *MockIPTables) ClearChain(table, chain string) error {
	for _, rule := range m.Rules {
		if rule.Table == table && rule.Chain == chain {
			err := m.Delete(rule.Table, rule.Chain, rule.RuleSpec...)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteChain deletes the given chain.
func (m *MockIPTables) DeleteChain(table, chain string) error {
	for i, ch := range m.Chains {
		if ch.Table == table && ch.Name == chain {
			m.Chains = append(m.Chains[:i], m.Chains[i+1:]...)
		}
	}
	return nil
}
