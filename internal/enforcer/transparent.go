package enforcer

import (
	"fmt"
	"strings"
)

// TransparentConfig defines parameters for transparent gateway mode.
type TransparentConfig struct {
	ProxyPort  int    // gateway listen port (default 3128)
	InboundIf  string // LAN-facing interface (e.g. "eth1")
	SubnetCIDR string // subnet to intercept (e.g. "10.0.0.0/24")
	TableName  string // nftables table name (default "clawgress_tproxy")
}

// RenderTransparentNft generates nftables rules for transparent gateway mode.
// HTTP (80) and HTTPS (443) traffic from the specified subnet is redirected
// to the local proxy port via DNAT. Non-HTTP traffic passes through.
func RenderTransparentNft(cfg TransparentConfig) string {
	if cfg.ProxyPort == 0 {
		cfg.ProxyPort = 3128
	}
	if cfg.TableName == "" {
		cfg.TableName = "clawgress_tproxy"
	}

	var sb strings.Builder
	sb.WriteString("# Transparent gateway mode — auto-generated, do not edit\n")
	sb.WriteString(fmt.Sprintf("table ip %s {\n", cfg.TableName))

	// NAT prerouting: redirect HTTP/HTTPS to proxy.
	sb.WriteString("  chain prerouting {\n")
	sb.WriteString("    type nat hook prerouting priority dstnat; policy accept;\n")
	if cfg.InboundIf != "" {
		sb.WriteString(fmt.Sprintf("    iifname != \"%s\" accept\n", cfg.InboundIf))
	}
	if cfg.SubnetCIDR != "" {
		sb.WriteString(fmt.Sprintf("    ip saddr != %s accept\n", cfg.SubnetCIDR))
	}
	sb.WriteString(fmt.Sprintf("    tcp dport { 80, 443 } redirect to :%d\n", cfg.ProxyPort))
	sb.WriteString("  }\n")

	// Forwarding: allow established and related, drop invalid.
	sb.WriteString("  chain forward {\n")
	sb.WriteString("    type filter hook forward priority filter; policy accept;\n")
	sb.WriteString("    ct state established,related accept\n")
	sb.WriteString("    ct state invalid drop\n")
	sb.WriteString("  }\n")

	// Postrouting: masquerade outbound.
	sb.WriteString("  chain postrouting {\n")
	sb.WriteString("    type nat hook postrouting priority srcnat; policy accept;\n")
	sb.WriteString("    masquerade\n")
	sb.WriteString("  }\n")

	sb.WriteString("}\n")
	return sb.String()
}
