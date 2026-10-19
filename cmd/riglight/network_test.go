package main

import (
	"net/netip"
	"testing"
)

func TestTailnetInterfaceRequiresAssignedUpTailscaleAddress(t *testing.T) {
	t.Parallel()

	listen := netip.MustParseAddr("100.64.0.10")
	for _, test := range []struct {
		name  string
		state tailnetInterfaceState
		want  bool
	}{
		{name: "assigned and up", state: tailnetInterfaceState{name: tailnetInterfaceName, up: true, addrs: []netip.Addr{listen}}, want: true},
		{name: "not up", state: tailnetInterfaceState{name: tailnetInterfaceName, addrs: []netip.Addr{listen}}},
		{name: "wrong interface", state: tailnetInterfaceState{name: "tun0", up: true, addrs: []netip.Addr{listen}}},
		{name: "not assigned", state: tailnetInterfaceState{name: tailnetInterfaceName, up: true, addrs: []netip.Addr{netip.MustParseAddr("100.64.0.20")}}},
		{name: "mapped assignment", state: tailnetInterfaceState{name: tailnetInterfaceName, up: true, addrs: []netip.Addr{netip.MustParseAddr("::ffff:100.64.0.10")}}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := tailnetInterfaceHasAddress(listen, test.state); got != test.want {
				t.Errorf("tailnetInterfaceHasAddress() = %t, want %t", got, test.want)
			}
		})
	}
}
