package enforcer

import (
	"strings"
	"testing"
)

func TestRenderTransparentDefaults(t *testing.T) {
	out := RenderTransparentNft(TransparentConfig{})
	if !strings.Contains(out, "table ip clawgress_tproxy") {
		t.Fatal("missing table")
	}
	if !strings.Contains(out, "redirect to :3128") {
		t.Fatal("missing redirect rule with default port")
	}
	if !strings.Contains(out, "masquerade") {
		t.Fatal("missing masquerade")
	}
}

func TestRenderTransparentWithInterface(t *testing.T) {
	out := RenderTransparentNft(TransparentConfig{
		InboundIf: "eth1",
		ProxyPort: 9999,
	})
	if !strings.Contains(out, `iifname != "eth1"`) {
		t.Fatal("missing interface filter")
	}
	if !strings.Contains(out, "redirect to :9999") {
		t.Fatal("missing custom port")
	}
}

func TestRenderTransparentWithSubnet(t *testing.T) {
	out := RenderTransparentNft(TransparentConfig{
		SubnetCIDR: "10.0.0.0/24",
		InboundIf:  "br0",
	})
	if !strings.Contains(out, "ip saddr != 10.0.0.0/24") {
		t.Fatal("missing subnet filter")
	}
}

func TestRenderTransparentChains(t *testing.T) {
	out := RenderTransparentNft(TransparentConfig{})
	for _, chain := range []string{"prerouting", "forward", "postrouting"} {
		if !strings.Contains(out, "chain "+chain) {
			t.Fatalf("missing chain %s", chain)
		}
	}
	if !strings.Contains(out, "ct state established,related accept") {
		t.Fatal("missing conntrack rule")
	}
	if !strings.Contains(out, "ct state invalid drop") {
		t.Fatal("missing invalid state drop")
	}
}
