package namespace

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type NetNS struct {
	Name   string
	Fd     int
	handle *netns.NsHandle
	loLink *netlink.Link
}

func New(name string) (*NetNS, error) {
	newHandle, err := netns.NewNamed(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create new named namespace: %w", err)
	}

	loLink, err := initLoopback()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize loopback interface: %w", err)
	}

	ns := &NetNS{Name: name, Fd: int(newHandle), handle: &newHandle, loLink: loLink}

	return ns, nil
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

func initLoopback() (*netlink.Link, error) {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return nil, fmt.Errorf("failed to get loopback interface: %w", err)
	}

	err = netlink.LinkSetUp(lo)
	if err != nil {
		return nil, fmt.Errorf("failed to set loopback interface up: %w", err)
	}

	return &lo, nil
}
