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
	handle *netns.NsHandle
}

func New(name string, router bool) (*NetNS, error) {
	newHandle, err := netns.NewNamed(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create new named namespace: %w", err)
	}

	ns := &NetNS{Name: name, Fd: int(newHandle), handle: &newHandle}

	if err := initLoopback(); err != nil {
		// Make sure to return the ns for cleanup purposes
		return ns, fmt.Errorf("failed to initialize loopback interface: %w", err)
	}

	if router {
		if err := ns.EnableIPv4Forwarding(); err != nil {
			return ns, fmt.Errorf("failed to enable IPv4 forwarding: %w", err)
		}
	}

	fmt.Printf("Namespace %s created successfully\n", ns.Name)

	return ns, nil
}

func (ns *NetNS) AddConnectivityLink(linkOriginalName string, linkDesignatedName string, addrIpNet *net.IPNet) error {
	// Get the link from root namespace
	// We assume by default the current scope is the root namespace.
	link, err := netlink.LinkByName(linkOriginalName)
	if err != nil {
		return fmt.Errorf("failed to get link by name: %w", err)
	}

	// Move the link to the target namespace using its file descriptor.
	if err := netlink.LinkSetNsFd(link, ns.Fd); err != nil {

		if errors.Is(err, syscall.EEXIST) {
			fmt.Printf("Link %s already exists in the namespace\n", linkOriginalName)
		}
		return fmt.Errorf("failed to assign endpoint %s to namespace: %w", linkOriginalName, err)
	}

	return helper.ExecuteInsideNamespace(ns.Fd, func() error {
		// Get the link again inside the new namespace
		link, err = netlink.LinkByName(linkOriginalName)
		if err != nil {
			return fmt.Errorf("failed to get link by name: %w", err)
		}

		// Rename the link if the original name differs from the designated name.
		if linkOriginalName != linkDesignatedName {
			if err := netlink.LinkSetName(link, linkDesignatedName); err != nil {
				return fmt.Errorf("failed to rename link from %s to %s: %w", linkOriginalName, linkDesignatedName, err)
			}
		}

		// Set the link address
		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: addrIpNet}); err != nil {
			return fmt.Errorf("failed to set link address: %w", err)
		}

		// Bring the link up
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring link up: %w", err)
		}

		fmt.Printf("\t- Link %s added to %s with address %s\n", linkDesignatedName, ns.Name, addrIpNet.String())

		return nil
	})
}

func (ns *NetNS) AddDefaultRoute(ifaceName string, gatewayAddr string, metric int) error {

	return helper.ExecuteInsideNamespace(ns.Fd, func() error {

		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("link %s not found in namespace: %w", ifaceName, err)
		}

		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       nil,
			Gw:        net.ParseIP(gatewayAddr), // Notice we don't use the gateway variable defined earlier since it becomes nil inside the function scope
			Priority:  metric,
		}

		if err := netlink.RouteAdd(route); err != nil && !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("failed to add default route to gateway %s: %w", gatewayAddr, err)
		}

		fmt.Printf("\t- Route added to %s via gateway %s with metric %d\n", ns.Name, gatewayAddr, metric)
		return nil
	})

}
func (ns *NetNS) AddRoute(
	ifaceName string,
	destinationCIDR string,
	gatewayAddr string,
	metric int,
) error {
	return helper.ExecuteInsideNamespace(ns.Fd, func() error {
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("link %s not found in namespace: %w", ifaceName, err)
		}

		_, dst, err := net.ParseCIDR(destinationCIDR)
		if err != nil {
			return fmt.Errorf("invalid destination CIDR %s: %w", destinationCIDR, err)
		}

		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       dst,
			Gw:        net.ParseIP(gatewayAddr),
			Priority:  metric,
		}

		if err := netlink.RouteAdd(route); err != nil && !errors.Is(err, syscall.EEXIST) {
			return fmt.Errorf("failed to add route to %s via %s: %w",
				destinationCIDR, gatewayAddr, err)
		}

		return nil
	})
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
