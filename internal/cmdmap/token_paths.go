package cmdmap

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type TokenPath struct {
	Kind        string   `json:"kind"`
	Tokens      []string `json:"tokens"`
	ValueToken  string   `json:"value_token,omitempty"`
	Description string   `json:"description"`
	Multi       bool     `json:"multi,omitempty"`
}

//go:embed command_schema.json
var commandSchemaJSON []byte

var TokenPaths = mustLoadTokenPaths()

func mustLoadTokenPaths() []TokenPath {
	var out []TokenPath
	if err := json.Unmarshal(commandSchemaJSON, &out); err != nil {
		panic(fmt.Sprintf("load command schema: %v", err))
	}
	return out
}

func SetTokenPaths() []TokenPath {
	out := make([]TokenPath, 0)
	for _, t := range TokenPaths {
		if t.Kind == "set" {
			out = append(out, t)
		}
	}
	return out
}

func ShowTokenPaths() []TokenPath {
	out := make([]TokenPath, 0)
	for _, t := range TokenPaths {
		if t.Kind == "show" {
			out = append(out, t)
		}
	}
	return out
}

func MatchSet(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	tokens := normalizeTokens(strings.Split(path, "."))
	for _, p := range SetTokenPaths() {
		if matchPatternTokens(normalizeTokens(p.Tokens), tokens) {
			return true
		}
	}
	return false
}

func IsMultiPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	tokens := normalizeTokens(strings.Split(path, "."))
	for _, p := range SetTokenPaths() {
		if p.Multi && matchPatternTokens(normalizeTokens(p.Tokens), tokens) {
			return true
		}
	}
	return false
}

func ValidateSetTokens(tokens []string, valueRaw string) (normalizedPath string, multi bool, err error) {
	nt := normalizeTokens(tokens)
	for _, tp := range SetTokenPaths() {
		pt := normalizeTokens(tp.Tokens)
		if !matchPatternTokens(pt, nt) {
			continue
		}
		if !validateValueToken(tp.ValueToken, valueRaw) {
			continue
		}
		return strings.Join(nt, "."), tp.Multi, nil
	}
	return "", false, fmt.Errorf("unknown or invalid command path/value")
}

func CommandSchemaStats() map[string]int {
	set := len(SetTokenPaths())
	show := len(ShowTokenPaths())
	return map[string]int{
		"set":   set,
		"show":  show,
		"total": set + show,
	}
}

func validateValueToken(token, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if token == "" {
		return true
	}
	if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
		inner := strings.Trim(token, "<>")
		if strings.Contains(inner, "|") {
			for _, alt := range strings.Split(inner, "|") {
				if value == alt {
					return true
				}
			}
			return false
		}
		switch inner {
		case "id", "port":
			return regexp.MustCompile(`^[0-9]+$`).MatchString(value)
		case "cidr":
			return regexp.MustCompile(`^[0-9a-fA-F:.]+/[0-9]{1,3}$`).MatchString(value)
		case "ip", "ipv4", "ipv6":
			return regexp.MustCompile(`^[0-9a-fA-F:.]+$`).MatchString(value)
		default:
			return true
		}
	}
	return value == token
}

func normalizeTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, strings.ReplaceAll(strings.TrimSpace(t), "-", "_"))
	}
	return out
}

func matchPatternTokens(pattern, value []string) bool {
	if len(pattern) != len(value) {
		return false
	}
	for i := range pattern {
		pt := pattern[i]
		vt := value[i]
		if strings.HasPrefix(pt, "<") && strings.HasSuffix(pt, ">") {
			if vt == "" {
				return false
			}
			continue
		}
		if strings.Contains(pt, "|") {
			alts := strings.Split(strings.Trim(pt, "<>"), "|")
			ok := false
			for _, alt := range alts {
				if vt == alt {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
			continue
		}
		if pt != vt {
			return false
		}
	}
	return true
}

func DedupSorted(paths []TokenPath) []TokenPath {
	seen := map[string]bool{}
	out := make([]TokenPath, 0, len(paths))
	for _, p := range paths {
		k := p.Kind + ":" + strings.Join(p.Tokens, "/") + ":" + p.ValueToken
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return strings.Join(a.Tokens, " ") < strings.Join(b.Tokens, " ")
	})
	return out
}
