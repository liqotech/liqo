package liqonet

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateIptablesChainsIfNotExist(t *testing.T) {
	chain := IPTableChain{
		Table: "nat",
		Name:  "testchain",
	}
	m := &MockIPTables{
		Rules:  []IPtableRule{},
		Chains: []IPTableChain{},
	}
	err := CreateIptablesChainsIfNotExist(m, chain.Table, chain.Name)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, true, m.containsChain(chain), "the chain should have been added")
	//we try to add again the same chain, nothing should happen and length of the chain slices should be 1
	err = CreateIptablesChainsIfNotExist(m, chain.Table, chain.Name)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, true, m.containsChain(chain), "the chain should have been added")
	assert.Equal(t, 1, len(m.Chains), "number of chains should be one")
}

func TestInsertIptablesRulespecIfNotExists(t *testing.T) {
	//test 1, the rule does not exist
	//expect for the rule to be inserted
	m := &MockIPTables{
		Rules:  []IPtableRule{},
		Chains: []IPTableChain{},
	}
	r := IPtableRule{
		Table:    "test",
		Chain:    "testChain",
		RuleSpec: []string{"test1", "test2"},
	}
	err := InsertIptablesRulespecIfNotExists(m, r.Table, r.Chain, r.RuleSpec)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, true, m.containsRule(r), "the rule should exist")
	//test 2, the rule exist and there are also other rules
	//expect that the rule is at the first position
	m = &MockIPTables{
		Rules: []IPtableRule{
			{
				Table:    "test",
				Chain:    "testChain",
				RuleSpec: []string{"test1", "test1"}},
			{
				Table:    "test",
				Chain:    "testChain",
				RuleSpec: []string{"test1", "test2"},
			}},
		Chains: []IPTableChain{},
	}
	r = IPtableRule{
		Table:    "test",
		Chain:    "testChain",
		RuleSpec: []string{"test1", "test2"},
	}
	err = InsertIptablesRulespecIfNotExists(m, r.Table, r.Chain, r.RuleSpec)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, true, m.containsRule(r), "the rule should exist")
	assert.Equal(t, r, m.Rules[0], "the new added rule should be the first one in the chain")
	assert.Equal(t, 2, len(m.Rules), "only two rules should be present in the chain")

	//test 3 multiple instances of the same rule are present
	//we expect that all the instances are removed and only one is left
	//at the first position
	m = &MockIPTables{
		Rules: []IPtableRule{
			{
				Table:    "test",
				Chain:    "testChain",
				RuleSpec: []string{"test1", "test1"}},
			{
				Table:    "test",
				Chain:    "testChain",
				RuleSpec: []string{"test1", "test2"},
			},
			{
				Table:    "test",
				Chain:    "testChain",
				RuleSpec: []string{"test1", "test2"},
			},
		},
		Chains: []IPTableChain{},
	}
	r = IPtableRule{
		Table:    "test",
		Chain:    "testChain",
		RuleSpec: []string{"test1", "test2"},
	}
	err = InsertIptablesRulespecIfNotExists(m, r.Table, r.Chain, r.RuleSpec)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, true, m.containsRule(r), "the rule should exist")
	assert.Equal(t, r, m.Rules[0], "the new added rule should be the first one in the chain")
	assert.Equal(t, 2, len(m.Rules), "only two rules should be present in the chain")
}
