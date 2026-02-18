package cmdmap

import (
	"testing"
)

func sampleForValue(token string) string {
	switch token {
	case "":
		return "value"
	case "<name>":
		return "edge"
	case "<server>":
		return "0.pool.ntp.org"
	case "<ifname>":
		return "eth0"
	case "<id>":
		return "100"
	case "<port>":
		return "443"
	case "<cidr>":
		return "10.0.0.0/24"
	case "<ip>", "<ipv4>":
		return "10.0.0.1"
	case "<ipv6>":
		return "2001:db8::1"
	case "<fqdn>":
		return "api.example.com"
	case "<value>":
		return "example"
	case "<allow|deny>":
		return "deny"
	case "<accept|drop>":
		return "drop"
	case "<tcp|udp|all>":
		return "tcp"
	case "<tcp|udp|icmp|all>":
		return "tcp"
	case "<tcp|http>":
		return "http"
	case "<lan|wan>":
		return "wan"
	default:
		if len(token) > 0 && token[0] == '<' {
			return "value"
		}
		return token
	}
}

func sampleForToken(token string) string {
	switch token {
	case "<ifname>":
		return "eth0"
	case "<id>":
		return "100"
	case "<name>":
		return "edge"
	default:
		if len(token) > 0 && token[0] == '<' {
			return "value"
		}
		return token
	}
}

func TestSchemaNoDuplicateSetPatterns(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SetTokenPaths() {
		k := p.Kind + ":" + join(p.Tokens) + ":" + p.ValueToken
		if seen[k] {
			t.Fatalf("duplicate command pattern: %s", k)
		}
		seen[k] = true
	}
}

func TestEverySetPathValidates(t *testing.T) {
	for _, p := range SetTokenPaths() {
		tokens := make([]string, 0, len(p.Tokens))
		for _, tok := range p.Tokens {
			tokens = append(tokens, sampleForToken(tok))
		}
		value := sampleForValue(p.ValueToken)
		path, _, err := ValidateSetTokens(tokens, value)
		if err != nil {
			t.Fatalf("validate failed for %v %s: %v", p.Tokens, value, err)
		}
		if !MatchSet(path) {
			t.Fatalf("match failed for normalized path: %s", path)
		}
	}
}

func TestUnknownSetRejected(t *testing.T) {
	if _, _, err := ValidateSetTokens([]string{"foo", "bar"}, "baz"); err == nil {
		t.Fatalf("expected unknown set to fail")
	}
}

func join(tokens []string) string {
	s := ""
	for i, t := range tokens {
		if i > 0 {
			s += " "
		}
		s += t
	}
	return s
}
