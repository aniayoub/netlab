package lab

import (
	"fmt"
	"os"
	"runtime"

	"github.com/aniayoub/netlab/internal/config"
	"github.com/aniayoub/netlab/internal/namespace"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func Setup(config *config.Config) error {
	// Lock the OS Thread throughout the entire setup process to ensure namespace operations are performed on the correct thread.
	// This should be enhanced later to lock only during critical namespace operations.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var nodes map[string]*namespace.NetNS
	var veths []*netlink.Veth

	var err error

	successful_setup := false

	// Store the currentNamespace network namespace so we can switch back to it later.
	currentNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer func() {
		if !successful_setup {
			setupCleanup(nodes, veths)
		}
	}()

	defer func() {
		fmt.Println("Restoring the original namespace...")

		if err := netns.Set(currentNamespace); err != nil {
			fmt.Println(fmt.Errorf("failed to restore the original namespace: %w", err))
		}
		if err := currentNamespace.Close(); err != nil {
			fmt.Println(fmt.Errorf("failed to close current namespace handle: %w", err))
		}
	}()

	nodes, err = setupNodes(config)
	if err != nil {
		return fmt.Errorf("failed to setup namespaces: %w", err)
	}

	// Switch back to the original namespace because each namespace creation does automatically switch to the new namespace.
	if err := netns.Set(currentNamespace); err != nil {
		return fmt.Errorf("failed to switch back to the original namespace: %w", err)
	}

	fmt.Println("Switched back to the original namespace successfully")

	veths, err = setupInterfaces(config, nodes)
	if err != nil {
		return fmt.Errorf("failed to setup interfaces: %w", err)
	}

	fmt.Println("Interfaces setup successfully")

	if err := setupRouting(config, nodes); err != nil {
		return fmt.Errorf("failed to setup routing: %w", err)
	}

	fmt.Println("Routing setup successfully")
	successful_setup = true

	return nil
}

func setupNodes(cfg *config.Config) (map[string]*namespace.NetNS, error) {
	createdNodes := make(map[string]*namespace.NetNS)
	for _, node := range cfg.Nodes {
		createdNode, err := namespace.New(node.Name, node.Type == config.Router)

		if createdNode != nil {
			createdNodes[node.Name] = createdNode
		}
		if err != nil {
			return createdNodes, fmt.Errorf("failed to create namespace for node %s: %w", node.Name, err)
		}
	}

	fmt.Println("Nodes setup successfully")

	return createdNodes, nil
}

func setupInterfaces(cfg *config.Config, namespaces map[string]*namespace.NetNS) ([]*netlink.Veth, error) {
	veths := []*netlink.Veth{}

	// We assume in the scope of this project that we won't have more than 26 nodes
	alphabet := "abcdefghijklmnopqrstuvwxyz"
	for i, link := range cfg.Links {
		veth, err := createVethPair(alphabet[i : i+1])
		if err != nil {
			return veths, fmt.Errorf("failed to create veth pair: %w", err)
		}

		veths = append(veths, veth)

		srcNs := namespaces[link.SrcNode]
		dstNs := namespaces[link.DstNode]

		srcNode := cfg.Nodes[link.SrcNode]
		dstNode := cfg.Nodes[link.DstNode]

		srcIface := srcNode.Interfaces[link.SrcInterface]
		dstIface := dstNode.Interfaces[link.DstInterface]

		// Add connectivity links for the source and destination interfaces
		if err := srcNs.AddConnectivityLink(veth.Name, link.SrcInterface, srcIface.IpNet); err != nil {
			return veths, fmt.Errorf("failed to add connectivity link for node %s interface %s: %w", link.SrcNode, link.SrcInterface, err)
		}

		if err := dstNs.AddConnectivityLink(veth.PeerName, link.DstInterface, dstIface.IpNet); err != nil {
			return veths, fmt.Errorf("failed to add connectivity link for node %s interface %s: %w", link.DstNode, link.DstInterface, err)
		}

	}

	fmt.Println("Interfaces setup successfully")
	return veths, nil
}

func createVethPair(suffix string) (*netlink.Veth, error) {
	name := fmt.Sprintf("eth-%s", suffix)
	peerName := fmt.Sprintf("port-%s", suffix)
	veth := &netlink.Veth{
		Name:     name,
		PeerName: peerName,
		MTU:      1500,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}
	return veth, nil
}

func setupRouting(cfg *config.Config, namespaces map[string]*namespace.NetNS) error {
	for _, node := range cfg.Nodes {
		ns := namespaces[node.Name]

		for i, route := range node.Routes {
			if err := ns.AddRoute(route.Interface, route.Destination, route.Gateway, (i+1)*100); err != nil {
				return fmt.Errorf("failed to add route for node %s: %w", node.Name, err)
			}
		}
	}
	/*
		for i, link := range cfg.Links {
			srcNs := namespaces[link.SrcNode]
			dstNs := namespaces[link.DstNode]

			srcNode := cfg.Nodes[link.SrcNode]
			dstNode := cfg.Nodes[link.DstNode]

			srcIface := srcNode.Interfaces[link.SrcInterface]
			dstIface := dstNode.Interfaces[link.DstInterface]

			if srcNode.Type == config.Host {
				// Make sure to assign the metric otherwise linux will not add two default routes with the same destination and metric.
				if err := srcNs.AddDefaultRoute(link.SrcInterface, dstIface.IpNet.IP.String(), (i+1)*100); err != nil {
					return fmt.Errorf("failed to add default route for node %s: %w", link.SrcNode, err)
				}
			}

			if dstNode.Type == config.Host {
				if err := dstNs.AddDefaultRoute(link.DstInterface, srcIface.IpNet.IP.String(), (i+1)*100); err != nil {
					return fmt.Errorf("failed to add default route for node %s: %w", link.DstNode, err)
				}
			}

			// Maybe for a transit link a concrete route would be a better choice
			// Since a router does not have default routes.
			// But for simplicity to establish connectivity, we add default routes between routers.
			// Otherwise, we will need to have destination subnet of each router
			if srcNode.Type == config.Router && dstNode.Type == config.Router {
				if err := srcNs.AddDefaultRoute(link.SrcInterface, dstIface.IpNet.IP.String(), (i+1)*100); err != nil {
					return fmt.Errorf("failed to add route for node %s: %w", link.SrcNode, err)
				}

				if err := dstNs.AddDefaultRoute(link.DstInterface, srcIface.IpNet.IP.String(), (i+1)*100); err != nil {
					return fmt.Errorf("failed to add route for node %s: %w", link.DstNode, err)
				}
			}

		}*/
	return nil
}

func setupCleanup(namespaces map[string]*namespace.NetNS, veths []*netlink.Veth) {
	fmt.Println("Cleaning up lab resources...")

	for _, veth := range veths {
		if veth != nil {
			if err := netlink.LinkDel(veth); err != nil {
				fmt.Printf("Failed to cleanup veth pair: %s\n", err.Error())
			}
		}
	}

	for name, ns := range namespaces {
		if ns != nil {
			if err := ns.Remove(); err != nil {
				fmt.Printf("Failed to remove namespace %s: %s\n", name, err.Error())
			}
		}
	}
}
func Remove(cfg *config.Config) error {
	for _, node := range cfg.Nodes {
		if err := netns.DeleteNamed(node.Name); err != nil {
			// If namespace doesn't exist, we can ignore the error.
			if os.IsNotExist(err) {
				fmt.Printf("Namespace %s does not exist, deletion considered successful\n", node.Name)
				continue
			}

			return fmt.Errorf("Error deleting namespace %s: %w", node.Name, err)
		}
		fmt.Printf("Namespace %s deleted successfully\n", node.Name)
	}

	fmt.Println("Lab removed successfully")
	return nil
}
