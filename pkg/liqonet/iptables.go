package liqonet

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
