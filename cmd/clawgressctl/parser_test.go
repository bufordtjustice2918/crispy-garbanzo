package main

import (
	"strings"
	"testing"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/cmdmap"
)

func sampleToken(token string) string {
	switch token {
	case "<ifname>":
		return "eth0"
	case "<id>":
		return "100"
	case "<name>":
		return "edge"
	default:
		if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
			inner := strings.Trim(token, "<>")
			if strings.Contains(inner, "|") {
				return strings.Split(inner, "|")[0]
			}
			return "value"
		}
		return token
	}
}

func sampleValue(token string) string {
	switch token {
	case "":
		return "value"
	case "<ifname>":
		return "eth0"
	case "<id>", "<port>":
		return "100"
	case "<cidr>":
		return "10.0.0.0/24"
	case "<ip>", "<ipv4>":
		return "10.0.0.1"
	case "<ipv6>":
		return "2001:db8::1"
	case "<fqdn>":
		return "api.example.com"
	case "<server>":
		return "0.pool.ntp.org"
	default:
		if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
			inner := strings.Trim(token, "<>")
			if strings.Contains(inner, "|") {
				return strings.Split(inner, "|")[0]
			}
			return "value"
		}
		return token
	}
}

func TestParseSetPathAndValueForAllSchemaSetPaths(t *testing.T) {
	for _, p := range cmdmap.SetTokenPaths() {
		args := make([]string, 0, len(p.Tokens)+1)
		for _, tok := range p.Tokens {
			args = append(args, sampleToken(tok))
		}
		args = append(args, sampleValue(p.ValueToken))

		path, _, err := parseSetPathAndValue(args)
		if err != nil {
			t.Fatalf("set parse failed for %v %q: %v", p.Tokens, p.ValueToken, err)
		}
		if !cmdmap.MatchSet(path) {
			t.Fatalf("parsed path did not match schema: %s", path)
		}
	}
}
