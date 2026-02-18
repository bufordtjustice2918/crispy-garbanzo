package cmdmap

import "testing"

func TestDHCPAddressBackendMapping(t *testing.T) {
	var found bool
	for _, c := range Commands()["set"] {
		if c.Pattern == "interfaces ethernet <ifname> address dhcp" {
			found = true
			if c.Backend != "dhclient+systemd" {
				t.Fatalf("expected dhclient+systemd backend, got %q", c.Backend)
			}
		}
	}
	if !found {
		t.Fatalf("dhcp address command not found in catalog")
	}
}
