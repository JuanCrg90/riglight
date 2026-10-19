package main

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
)

const tailnetInterfaceName = "tailscale0"

var (
	tailnetIPv4 = netip.MustParsePrefix("100.64.0.0/10")
	tailnetIPv6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

type tailnetInterfaceState struct {
	name  string
	up    bool
	addrs []netip.Addr
}

type stringValues []string

func (values *stringValues) String() string {
	return strings.Join(*values, ",")
}

func (values *stringValues) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func canonicalTailnetListen(address string) (string, netip.Addr, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", netip.Addr{}, false
	}
	ip, ok := canonicalTailnetAddr(host)
	if !ok {
		return "", netip.Addr{}, false
	}
	canonicalPort, ok := canonicalPort(port)
	if !ok {
		return "", netip.Addr{}, false
	}
	return net.JoinHostPort(ip.String(), canonicalPort), ip, true
}

func canonicalTailnetHost(authority, listenPort string) (string, bool) {
	host, port, err := net.SplitHostPort(authority)
	if err != nil || port != listenPort || !validMagicDNSName(host) {
		return "", false
	}
	return net.JoinHostPort(host, port), true
}

func canonicalTailnetAddr(value string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	ip = ip.Unmap()
	if !tailnetIPv4.Contains(ip) && !tailnetIPv6.Contains(ip) {
		return netip.Addr{}, false
	}
	return ip, true
}

func canonicalPort(port string) (string, bool) {
	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 || value > 65535 {
		return "", false
	}
	return strconv.Itoa(value), true
}

func validMagicDNSName(host string) bool {
	if host == "" || len(host) > 253 || host == "ts.net" || !strings.HasSuffix(host, ".ts.net") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func lookupTailnetInterface() (tailnetInterfaceState, error) {
	interfaceValue, err := net.InterfaceByName(tailnetInterfaceName)
	if err != nil {
		return tailnetInterfaceState{}, err
	}
	addresses, err := interfaceValue.Addrs()
	if err != nil {
		return tailnetInterfaceState{}, err
	}
	state := tailnetInterfaceState{
		name: tailnetInterfaceName,
		up:   interfaceValue.Flags&net.FlagUp != 0,
	}
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil {
			return tailnetInterfaceState{}, err
		}
		state.addrs = append(state.addrs, prefix.Addr().Unmap())
	}
	return state, nil
}

func tailnetInterfaceHasAddress(listen netip.Addr, state tailnetInterfaceState) bool {
	listen = listen.Unmap()
	if state.name != tailnetInterfaceName || !state.up || (!tailnetIPv4.Contains(listen) && !tailnetIPv6.Contains(listen)) {
		return false
	}
	for _, address := range state.addrs {
		if listen == address.Unmap() {
			return true
		}
	}
	return false
}
