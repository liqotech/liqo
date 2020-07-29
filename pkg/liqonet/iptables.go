package liqonet

import (
	"fmt"
	"strings"
)

type IPtableRule struct {
	Table    string
	Chain    string
	RuleSpec []string
}

type IPTableChain struct {
	Table string
	Name  string
}

type IPTables interface {
	/*AppendUnique(table string, chain string, rulespec ...string) error*/
	Insert(table string, chain string, pos int, rulespec ...string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
	ListChains(table string) ([]string, error)
	NewChain(table string, chain string) error
	List(table, chain string) ([]string, error)
	AppendUnique(table string, chain string, rulespec ...string) error
	ClearChain(table, chain string) error
	DeleteChain(table, chain string) error
}

//We create the chains in two different tables.
//LIQONET-POSTROUTING is created in the nat table
//LIQONET-FORWARD is created in the filter table
func CreateIptablesChainsIfNotExist(ipt IPTables, table string, newChain string) error {
	//get existing chains
	chains_list, err := ipt.ListChains(table)
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
	err = ipt.NewChain(table, newChain)
	if err != nil {
		return fmt.Errorf("unable to create %s chain in %s table: %v", newChain, table, err)
	}
	return nil
}

//this function is used to insert the rules that forward the traffic to the specific chains.
//it takes care that the rule is present only once and at the same time it inserts it at the first position
//TODO: a go routine which periodically checks if the rules inserted with this function in position one in the chain where belong
func InsertIptablesRulespecIfNotExists(ipt IPTables, table string, chain string, ruleSpec []string) error {
	//get the list of rulespecs for the specified chain
	rulesList, err := ipt.List(table, chain)
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
			if err = ipt.Delete(table, chain, ruleSpec...); err != nil {
				return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
			}
		}
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
	} else if numOccurrences == 1 {
		//if the occurrence is one then check the position and if not at the first one we delete and reinsert it
		if strings.Contains(rulesList[0], strings.Join(ruleSpec, " ")) {
			return nil
		}
		if err = ipt.Delete(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("unable to delete iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
		return nil
	} else if numOccurrences == 0 {
		//if the occurrence is zero then insert the rule in first position
		if err = ipt.Insert(table, chain, 1, ruleSpec...); err != nil {
			return fmt.Errorf("unable to inserte iptable rule \"%s\": %v", strings.Join(ruleSpec, " "), err)
		}
	}
	return nil
}
