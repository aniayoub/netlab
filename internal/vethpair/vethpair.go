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
	veth netlink.Veth
}

type Endpoint string

const (
	EndpointA Endpoint = "veth0"
	EndpointB Endpoint = "veth1"
)

func New(fdA int, fdB int) (*VethPair, error) {
	endpointA := string(EndpointA)
	endpointB := string(EndpointB)

	veth, err := initVeth(endpointA, endpointB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize veth pair: %w", err)
	}

	v := &VethPair{veth: *veth}

	if err := assignLinkToNs(EndpointA, fdA); err != nil {
		return nil, err
	}

	if err := assignLinkToNs(EndpointB, fdB); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *VethPair) Configure(nsFD int, endpoint Endpoint, address string) error {

	originalNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer originalNs.Close()

	if nsFD != int(originalNs) {
		if err := netns.Set(netns.NsHandle(nsFD)); err != nil {
			return fmt.Errorf("failed to switch to namespace: %w", err)
		}
	}
	defer netns.Set(originalNs)

	link, err := netlink.LinkByName(string(endpoint))
	if err != nil {
		return fmt.Errorf("failed to get link by name %s: %w", endpoint, err)
	}

	if err := configureAddress(&link, address); err != nil {
		return fmt.Errorf("failed to configure address for endpoint %s: %w", endpoint, err)
	}

	fmt.Printf("Configured endpoint %s with address %s\n", endpoint, address)

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring endpoint %s up: %w", endpoint, err)
	}

	fmt.Printf("Endpoint %s brought up\n", endpoint)

	return nil
}

func configureAddress(link *netlink.Link, address string) error {
	ipNet, err := netlink.ParseIPNet(address)
	if err != nil {
		return fmt.Errorf("failed to parse IP network: %w", err)
	}

	return netlink.AddrAdd(*link, &netlink.Addr{IPNet: ipNet})
}

func assignLinkToNs(endpoint Endpoint, nsFD int) error {
	link, err := netlink.LinkByName(string(endpoint))
	if err != nil {
		return fmt.Errorf("failed to get link by name %s: %w", endpoint, err)
	}
	if err := netlink.LinkSetNsFd(link, nsFD); err != nil {

		if errors.Is(err, syscall.EEXIST) {
			fmt.Printf("Link %s already exists in the namespace\n", endpoint)
		}
		return fmt.Errorf("failed to assign endpoint %s to namespace: %w", endpoint, err)
	}
	return nil
}

func (v *VethPair) Cleanup() error {
	// Note: This action depends on the current namespace.
	// If the links of veth are part of a deffierent namespace this will fail
	// However this can be accepted for now in the scope of this project because removing the namespaces will result in removing the veht pair
	if err := netlink.LinkDel(&v.veth); err != nil {
		return fmt.Errorf("failed to delete veth link: %w", err)
	}
	return nil
}

func initVeth(endpointA string, endpointB string) (*netlink.Veth, error) {

	veth := netlink.Veth{
		Name:     endpointA,
		MTU:      1500,
		PeerName: endpointB,
	}

	if err := netlink.LinkAdd(&veth); err != nil {
		return nil, fmt.Errorf("failed to add veth link: %w", err)
	}
	return &veth, nil
}
