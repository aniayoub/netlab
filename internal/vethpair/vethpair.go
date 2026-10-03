package vethpair

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// VethPair represents a virtual Ethernet pair with two endpoints.
type VethPair struct {
	HostNsFd int
	veth     netlink.Veth
	port     netlink.Link
}

func New(hostNsFd int, bridgeIndex int, keyChar string) (*VethPair, error) {
	hostName := "eth0"
	portName := fmt.Sprintf("port-%s", keyChar)

	// Initialise a new veth that belongs to the host and the port belonging to the bridge
	veth, peer, err := initVeth(hostName, portName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize veth pair: %w", err)
	}

	v := &VethPair{veth: *veth, port: *peer, HostNsFd: hostNsFd}

	if err := v.assignPeerToBridge(peer, bridgeIndex); err != nil {
		return v, fmt.Errorf("failed to bring veth link up: %w", err)
	}

	if err := assignLinkToNs(hostName, hostNsFd); err != nil {
		return v, err
	}

	return v, nil
}

func (v *VethPair) Configure(address string) error {

	originalNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer originalNs.Close()

	if v.HostNsFd != int(originalNs) {
		if err := netns.Set(netns.NsHandle(v.HostNsFd)); err != nil {
			return fmt.Errorf("failed to switch to namespace: %w", err)
		}
	}
	defer netns.Set(originalNs)

	link, err := netlink.LinkByName(v.veth.Name)
	if err != nil {
		return fmt.Errorf("failed to get link by name %s: %w", v.veth.Name, err)
	}

	if err := configureAddress(&link, address); err != nil {
		return fmt.Errorf("failed to configure address for endpoint %s: %w", v.veth.Name, err)
	}

	fmt.Printf("Configured endpoint %s with address %s\n", v.veth.Name, address)

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring endpoint %s up: %w", v.veth.Name, err)
	}

	fmt.Printf("Endpoint %s brought up\n", v.veth.Name)

	return nil
}

func configureAddress(link *netlink.Link, address string) error {
	ipNet, err := netlink.ParseIPNet(address)
	if err != nil {
		return fmt.Errorf("failed to parse IP network: %w", err)
	}

	return netlink.AddrAdd(*link, &netlink.Addr{IPNet: ipNet})
}

func assignLinkToNs(linkName string, nsFD int) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link by name %s: %w", linkName, err)
	}
	if err := netlink.LinkSetNsFd(link, nsFD); err != nil {

		if errors.Is(err, syscall.EEXIST) {
			fmt.Printf("Link %s already exists in the namespace\n", linkName)
		}
		return fmt.Errorf("failed to assign endpoint %s to namespace: %w", linkName, err)
	}
	return nil
}

func (v *VethPair) assignPeerToBridge(link *netlink.Link, bridgeIndex int) error {
	linkName := (*link).Attrs().Name

	if err := netlink.LinkSetMasterByIndex(*link, bridgeIndex); err != nil {
		return fmt.Errorf("failed to set master for link %s: %w", linkName, err)
	}

	if err := netlink.LinkSetUp(*link); err != nil {
		return fmt.Errorf("failed to bring link %s up: %w", linkName, err)
	}

	return nil
}

func (v *VethPair) Cleanup() error {
	// We make the port the authorative link since it belongs the root namespace
	// If we choose the eth link then we might try to delete a link that is in a namespace that is already gone
	// We also assume cleanup will be called from the root namespace

	fmt.Printf("Cleaning up veth pair with port %s\n", v.port.Attrs().Name)
	err := netlink.LinkDel(v.port)

	// Defferentiate between a link that doesn't exist and other errors
	if err != nil {
		if errors.Is(err, syscall.ENODEV) {
			fmt.Printf("Link %s does not exist, nothing to clean up\n", v.port.Attrs().Name)
			return nil
		}
		return fmt.Errorf("failed to delete veth link: %w", err)
	}
	return nil
}

func initVeth(name string, peerName string) (*netlink.Veth, *netlink.Link, error) {

	veth := netlink.Veth{
		Name:     name,
		MTU:      1500,
		PeerName: peerName,
	}

	if err := netlink.LinkAdd(&veth); err != nil {
		return nil, nil, fmt.Errorf("failed to add veth link: %w", err)
	}

	link, err := netlink.LinkByName(peerName)

	if err != nil {
		if err := netlink.LinkDel(&veth); err != nil {
			fmt.Printf("Failed to delete veth link after error: %v\n", err)
		}
		return nil, nil, fmt.Errorf("failed to get peer link by name %s: %w", peerName, err)
	}
	return &veth, &link, nil

}
