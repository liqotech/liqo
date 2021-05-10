package liqonet

import (
	"strings"
)

type MockIPTables struct {
	Rules  []IPtableRule
	Chains []IPTableChain
}

func (m *MockIPTables) Exists(table string, chain string, rulespec ...string) (bool, error) {
	for _, rule := range m.Rules {
		if rule.Table == table && rule.Chain == chain && strings.Join(rule.RuleSpec, " ") == strings.Join(rulespec, " ") {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockIPTables) AppendUnique(table string, chain string, rulespec ...string) error {
	r := IPtableRule{
		Table:    table,
		Chain:    chain,
		RuleSpec: rulespec,
	}
	if m.containsRule(r) {
		return nil
	} else {
		m.Rules = append(m.Rules, r)
		return nil
	}
}

func (m *MockIPTables) ListChains(table string) ([]string, error) {
	var chains []string
	for _, chain := range m.Chains {
		if chain.Table == table {
			chains = append(chains, chain.Name)
		}
	}
	return chains, nil
}

func (m *MockIPTables) NewChain(table string, chain string) error {
	m.Chains = append(m.Chains, IPTableChain{
		Table: table,
		Name:  chain,
	})
	return nil
}

func (m *MockIPTables) containsChain(chain IPTableChain) bool {
	for _, ch := range m.Chains {
		if ch.Table == chain.Table && ch.Name == chain.Name {
			return true
		}
	}
	return false
}

func (m *MockIPTables) ruleIndex(table string, chain string, rulespec []string) int {
	for i, rule := range m.Rules {
		if rule.Table == table && rule.Chain == chain && strings.Join(rule.RuleSpec, " ") == strings.Join(rulespec, " ") {
			return i
		}
	}
	return -1
}

func (m *MockIPTables) chainIndex(table string, name string) int {
	for i, chain := range m.Chains {
		if chain.Table == table && chain.Name == name {
			return i
		}
	}
	return -1
}

func (m *MockIPTables) containsRule(rule IPtableRule) bool {
	for _, r := range m.Rules {
		if r.Table == rule.Table && r.Chain == rule.Chain && strings.Join(r.RuleSpec, " ") == strings.Join(rule.RuleSpec, " ") {
			return true
		}
	}
	return false
}

func (m *MockIPTables) Delete(table string, chain string, rulespec ...string) error {
	var ruleIndex = m.ruleIndex(table, chain, rulespec)
	if ruleIndex != -1 {
		m.Rules = append(m.Rules[:ruleIndex], m.Rules[ruleIndex+1:]...)
	}
	return nil
}

func (m *MockIPTables) prependRule(x []IPtableRule, y IPtableRule) []IPtableRule {
	x = append(x, IPtableRule{})
	copy(x[1:], x)
	x[0] = y
	return x
}

//this mock function prepends even if the index is different than 1.
func (m *MockIPTables) Insert(table string, chain string, pos int, rulespec ...string) error {
	m.Rules = m.prependRule(m.Rules, IPtableRule{
		Table:    table,
		Chain:    chain,
		RuleSpec: rulespec,
	})
	return nil
}

func (m *MockIPTables) List(table, chain string) ([]string, error) {
	var rules []string
	for _, rule := range m.Rules {
		if rule.Table == table {
			rules = append(rules, strings.Join(rule.RuleSpec, " "))
		}
	}
	return rules, nil
}

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

func (m *MockIPTables) DeleteChain(table, chain string) error {
	for i, ch := range m.Chains {
		if ch.Table == table && ch.Name == chain {
			m.Chains = append(m.Chains[:i], m.Chains[i+1:]...)
		}
	}
	return nil
}
