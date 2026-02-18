package cmdmap

import "strings"

type Command struct {
	Kind        string `json:"kind"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Backend     string `json:"backend"`
	Example     string `json:"example"`
}

func Commands() map[string][]Command {
	set := make([]Command, 0)
	show := make([]Command, 0)
	for _, tp := range TokenPaths {
		cmd := Command{
			Kind:        tp.Kind,
			Pattern:     tokensToPattern(tp),
			Description: tp.Description,
			Backend:     backendForTokens(tp.Tokens),
			Example:     exampleFor(tp),
		}
		if tp.Kind == "set" {
			set = append(set, cmd)
		} else if tp.Kind == "show" {
			show = append(show, cmd)
		}
	}
	return map[string][]Command{"set": set, "show": show}
}

func tokensToPattern(tp TokenPath) string {
	parts := append([]string{}, tp.Tokens...)
	if tp.ValueToken != "" {
		parts = append(parts, tp.ValueToken)
	}
	return strings.Join(parts, " ")
}

func exampleFor(tp TokenPath) string {
	tokens := make([]string, 0, len(tp.Tokens)+1)
	for _, t := range tp.Tokens {
		tokens = append(tokens, sampleToken(t))
	}
	if tp.ValueToken != "" {
		tokens = append(tokens, sampleValue(tp.ValueToken))
	}
	if tp.Kind == "show" {
		return "show " + strings.Join(tokens, " ")
	}
	return "set " + strings.Join(tokens, " ")
}

func sampleToken(t string) string {
	if strings.HasPrefix(t, "<") && strings.HasSuffix(t, ">") {
		s := strings.Trim(t, "<>")
		if strings.Contains(s, "|") {
			return strings.Split(s, "|")[0]
		}
		switch s {
		case "ifname":
			return "eth0"
		case "id":
			return "100"
		case "name":
			return "edge"
		default:
			return "value"
		}
	}
	return t
}

func sampleValue(v string) string {
	if strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">") {
		s := strings.Trim(v, "<>")
		if strings.Contains(s, "|") {
			return strings.Split(s, "|")[0]
		}
		switch s {
		case "port", "id":
			return "100"
		case "cidr":
			return "10.0.0.0/24"
		case "ip", "ipv4":
			return "10.0.0.1"
		case "ipv6":
			return "2001:db8::1"
		case "fqdn":
			return "api.example.com"
		case "ifname":
			return "eth0"
		default:
			return "value"
		}
	}
	return v
}

func backendForTokens(tokens []string) string {
	if len(tokens) == 0 {
		return "config"
	}
	joined := strings.Join(tokens, ".")
	switch {
	case strings.HasPrefix(joined, "firewall.nftables") || strings.HasPrefix(joined, "nat."):
		return "nftables"
	case strings.HasPrefix(joined, "service.dns"):
		return "bind9"
	case strings.HasPrefix(joined, "service.haproxy"):
		return "haproxy"
	case strings.HasPrefix(joined, "system.ntp"):
		return "chrony"
	case strings.HasPrefix(joined, "interfaces."):
		return "systemd-networkd"
	default:
		return "control-plane"
	}
}
