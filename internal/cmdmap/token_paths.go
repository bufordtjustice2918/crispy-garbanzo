package cmdmap

type TokenPath struct {
	Kind        string   `json:"kind"`
	Tokens      []string `json:"tokens"`
	ValueToken  string   `json:"value_token,omitempty"`
	Description string   `json:"description"`
}

var TokenPaths = []TokenPath{
	{Kind: "set", Tokens: []string{"system", "host-name"}, ValueToken: "<name>", Description: "Set system hostname"},
	{Kind: "set", Tokens: []string{"system", "ntp", "server"}, ValueToken: "<server>", Description: "Append NTP server"},
	{Kind: "set", Tokens: []string{"interfaces", "ethernet", "<ifname>", "address"}, ValueToken: "<cidr>", Description: "Set interface address"},
	{Kind: "set", Tokens: []string{"interfaces", "ethernet", "<ifname>", "role"}, ValueToken: "<lan|wan>", Description: "Set interface role"},
	{Kind: "set", Tokens: []string{"firewall", "nftables", "input", "default-action"}, ValueToken: "<accept|drop>", Description: "Set input chain policy"},
	{Kind: "set", Tokens: []string{"firewall", "nftables", "forward", "default-action"}, ValueToken: "<accept|drop>", Description: "Set forward chain policy"},
	{Kind: "set", Tokens: []string{"firewall", "nftables", "wan-block"}, ValueToken: "enable", Description: "Enable WAN block profile"},
	{Kind: "set", Tokens: []string{"firewall", "group", "address-group", "<name>", "address"}, ValueToken: "<cidr>", Description: "Append address-group member"},
	{Kind: "set", Tokens: []string{"nat", "source", "rule", "<id>", "outbound-interface"}, ValueToken: "<ifname>", Description: "Set source NAT outbound interface"},
	{Kind: "set", Tokens: []string{"nat", "source", "rule", "<id>", "source", "address"}, ValueToken: "<cidr>", Description: "Set source NAT selector"},
	{Kind: "set", Tokens: []string{"nat", "source", "rule", "<id>", "translation", "address"}, ValueToken: "masquerade", Description: "Enable masquerade translation"},
	{Kind: "set", Tokens: []string{"service", "dns", "forwarding", "listen-address"}, ValueToken: "<ip>", Description: "Set DNS forwarder listen address"},
	{Kind: "set", Tokens: []string{"service", "dns", "forwarding", "allow-from"}, ValueToken: "<cidr>", Description: "Append DNS allowed client range"},
	{Kind: "set", Tokens: []string{"service", "haproxy"}, ValueToken: "enable", Description: "Enable HAProxy service"},
	{Kind: "set", Tokens: []string{"service", "haproxy", "stats", "port"}, ValueToken: "<port>", Description: "Set HAProxy stats port"},
	{Kind: "set", Tokens: []string{"policy", "egress", "default-action"}, ValueToken: "<allow|deny>", Description: "Set default egress decision"},
	{Kind: "set", Tokens: []string{"policy", "egress", "allow-domain"}, ValueToken: "<fqdn>", Description: "Append allowed destination domain"},
	{Kind: "set", Tokens: []string{"policy", "egress", "deny-domain"}, ValueToken: "<fqdn>", Description: "Append denied destination domain"},

	{Kind: "show", Tokens: []string{"commands"}, Description: "List supported command catalog"},
	{Kind: "show", Tokens: []string{"configuration"}, Description: "Show candidate configuration JSON"},
	{Kind: "show", Tokens: []string{"configuration", "commands"}, Description: "Show candidate as set commands"},
	{Kind: "show", Tokens: []string{"system", "ntp"}, Description: "Show NTP command space"},
	{Kind: "show", Tokens: []string{"interfaces"}, Description: "Show interface subtree"},
	{Kind: "show", Tokens: []string{"firewall"}, Description: "Show firewall subtree"},
	{Kind: "show", Tokens: []string{"nat", "source", "rules"}, Description: "Show source NAT subtree"},
	{Kind: "show", Tokens: []string{"service", "dns"}, Description: "Show DNS service subtree"},
	{Kind: "show", Tokens: []string{"service", "haproxy"}, Description: "Show HAProxy service subtree"},
}
