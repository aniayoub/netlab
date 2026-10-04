package namespace

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/aniayoub/netlab/internal/helper"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type NetNS struct {
	Name   string
	Fd     int
	router bool
	handle *netns.NsHandle
}

func New(name string, router bool) (*NetNS, error) {
	newHandle, err := netns.NewNamed(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create new named namespace: %w", err)
	}

	ns := &NetNS{Name: name, Fd: int(newHandle), router: router, handle: &newHandle}

	if err := initLoopback(); err != nil {
		// Make sure to return the ns for cleanup purposes
		return ns, fmt.Errorf("failed to initialize loopback interface: %w", err)
	}

	if router {
		if err := ns.EnableIPv4Forwarding(); err != nil {
			return ns, fmt.Errorf("failed to enable IPv4 forwarding: %w", err)
		}
	}

	return ns, nil
}

func (ns *NetNS) AddConnectivityLink(linkName string, address []int) error {
	if len(address) != 4 {
		return fmt.Errorf("invalid address length: expected 4, got %d", len(address))
	}

	if !ns.router && address[3] == 1 {
		return fmt.Errorf("address ending with 1 is reserved to default")
	}

	if address[3] == 0 {
		return fmt.Errorf("address ending with 0 is reserved to network")
	}

	// First move the link to the current namespace
	// We assume the link is already created and exists in the current namespace.
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link by name: %w", err)
	}
	if err := netlink.LinkSetNsFd(link, ns.Fd); err != nil {

		if errors.Is(err, syscall.EEXIST) {
			fmt.Printf("Link %s already exists in the namespace\n", linkName)
		}
		return fmt.Errorf("failed to assign endpoint %s to namespace: %w", linkName, err)
	}

	return helper.ExecuteInsideNamespace(ns.Fd, func() error {
		// Get the link again inside the new namespace
		link, err = netlink.LinkByName(linkName)
		if err != nil {
			return fmt.Errorf("failed to get link by name: %w", err)
		}

		// Set the link address
		addrStr := fmt.Sprintf("%d.%d.%d.%d/24", address[0], address[1], address[2], address[3])
		if err := configureAddress(&link, addrStr); err != nil {
			return fmt.Errorf("failed to set link address: %w", err)
		}

		// Bring the link up
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring link up: %w", err)
		}

		if !ns.router {
			if err := netlink.LinkSetName(link, "eth0"); err != nil {
				return fmt.Errorf("failed to rename link to default eth0: %w", err)
			}
		}

		fmt.Printf("\t- Link %s added to %s with address %s\n", linkName, ns.Name, addrStr)

		return nil
	})

}

func (ns *NetNS) AddDefaultRoute(gatewayAddr string) error {

	return helper.ExecuteInsideNamespace(ns.Fd, func() error {

		link, err := netlink.LinkByName("eth0")
		if err != nil {
			return fmt.Errorf("link %s not found in namespace: %w", "eth0", err)
		}

		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       nil,
			Gw:        net.ParseIP(gatewayAddr), // Notice we don't use the gateway variable defined earlier since it becomes nil inside the function scope
		}

		if err := netlink.RouteAdd(route); err != nil && !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("failed to add default route: %w", err)
		}

		fmt.Printf("\t- Route added to %s via gateway %s\n", ns.Name, gatewayAddr)
		return nil
	})

}

func configureAddress(link *netlink.Link, address string) error {
	ipNet, err := netlink.ParseIPNet(address)
	if err != nil {
		return fmt.Errorf("failed to parse IP network: %w", err)
	}

	return netlink.AddrAdd(*link, &netlink.Addr{IPNet: ipNet})
}
func (ns *NetNS) Remove() error {
	fmt.Print("Removing namespace: ", ns.Name, "\n")
	// Close the handle before deleting the namespace
	if ns.handle.IsOpen() {
		// We will accept any errors that occur while closing the handle.
		// Deleting the namespace below should evetually succeed even if closing the handle fails.
		if err := ns.handle.Close(); err != nil {
			fmt.Println(fmt.Errorf("failed to close namespace handle: %w", err))
		}
	}

	ok := netns.DeleteNamed(ns.Name)
	if ok != nil {
		return fmt.Errorf("failed to delete named namespace: %w", ok)
	}
	return nil
}

func initLoopback() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("failed to get loopback interface: %w", err)
	}

	err = netlink.LinkSetUp(lo)
	if err != nil {
		return fmt.Errorf("failed to set loopback interface up: %w", err)
	}

	return nil
}

func (ns *NetNS) EnableIPv4Forwarding() error {
	// This is called after initializing the namespace so we are still in the scope of the namespace.
	err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to enable IPv4 forwarding: %w", err)
	}
	fmt.Printf("IP forwarding enabled in %s!\n", ns.Name)
	return nil
}
