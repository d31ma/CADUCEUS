package mailauth

import (
	"net"
	"slices"
)

type spfNetwork struct {
	remote  net.IP
	prefix4 int
	prefix6 int
	limits  spfLimits
}

func (network spfNetwork) coversAny(addresses []net.IP) bool {
	return slices.ContainsFunc(addresses, network.covers)
}

func (network spfNetwork) covers(address net.IP) bool {
	block := net.IPNet{IP: address, Mask: network.mask(address)}
	return block.Contains(network.remote)
}

func (network spfNetwork) mask(address net.IP) net.IPMask {
	if address.To4() != nil {
		return net.CIDRMask(network.prefix4, network.limits.bits4())
	}
	return net.CIDRMask(network.prefix6, network.limits.bits6())
}
