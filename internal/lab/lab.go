package lab

import (
	"fmt"
	"runtime"

	"github.com/aniayoub/netlab/internal/namespace"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func Setup() error {
	// Lock the OS Thread throughout the entire setup process to ensure namespace operations are performed on the correct thread.
	// This should be enhanced later to lock only during critical namespace operations.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var namespaceA, namespaceB, router *namespace.NetNS
	var vethA, vethB *netlink.Veth

	var err error

	successful_setup := false

	// Store the currentNamespace network namespace so we can switch back to it later.
	currentNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer func() {
		if !successful_setup {
			setupCleanup(namespaceA, namespaceB, router, vethA, vethB)
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

	namespaceA, namespaceB, router, err = setupNamespace()

	if err != nil {
		return fmt.Errorf("failed to setup namespaces: %w", err)
	}

	// Switch back to the original namespace because each namespace creation does automatically switch to the new namespace.
	if err := netns.Set(currentNamespace); err != nil {
		return fmt.Errorf("failed to switch back to the original namespace: %w", err)
	}

	fmt.Println("Switched back to the original namespace successfully")

	vethA, vethB, err = setupInterfaces(namespaceA, namespaceB, router)
	if err != nil {
		return fmt.Errorf("failed to setup interfaces: %w", err)
	}

	fmt.Println("Interfaces setup successfully")

	err = setupRouting(namespaceA, namespaceB)
	if err != nil {
		return fmt.Errorf("failed to setup routing: %w", err)
	}
	fmt.Println("Routing setup successfully")

	successful_setup = true

	return nil
}

func setupNamespace() (namespaceA *namespace.NetNS, namespaceB *namespace.NetNS, router *namespace.NetNS, err error) {
	// create namespaces A
	// NOTE: This step automatically switches to the new namespace.
	namespaceA, err = namespace.New("host-a", false)

	if err != nil {
		err = fmt.Errorf("failed to setup namespace host-a: %w", err)
		return
	}

	fmt.Println("Namespace host-a setup successfully")

	// create namespaces B
	// NOTE: This step automatically switches to the new namespace.
	namespaceB, err = namespace.New("host-b", false)
	if err != nil {
		err = fmt.Errorf("failed to setup namespace host-b: %w", err)
		return
	}

	fmt.Println("Namespace host-b setup successfully")

	// create router namespace
	// NOTE: This step automatically switches to the new namespace.
	router, err = namespace.New("router", true)
	if err != nil {
		err = fmt.Errorf("failed to setup namespace router: %w", err)
		return
	}

	fmt.Println("Namespace router setup successfully")

	return namespaceA, namespaceB, router, err
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

func setupInterfaces(namespaceA, namespaceB, router *namespace.NetNS) (vethA, vethB *netlink.Veth, err error) {

	vethA, err = createVethPair("a")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup veth pair: %w", err)
	}

	vethB, err = createVethPair("b")
	if err != nil {
		return vethA, nil, fmt.Errorf("failed to setup veth pair: %w", err)
	}

	err = router.AddConnectivityLink(vethA.PeerName, []int{10, 0, 1, 1})
	if err != nil {
		return vethA, vethB, fmt.Errorf("failed to add link to router for vethA: %w", err)
	}

	err = router.AddConnectivityLink(vethB.PeerName, []int{10, 0, 2, 1})
	if err != nil {
		return vethA, vethB, fmt.Errorf("failed to add link to router for vethB: %w", err)
	}

	err = namespaceA.AddConnectivityLink(vethA.Name, []int{10, 0, 1, 2})
	if err != nil {
		return vethA, vethB, fmt.Errorf("failed to add link to namespace A: %w", err)
	}

	err = namespaceB.AddConnectivityLink(vethB.Name, []int{10, 0, 2, 2})
	if err != nil {
		return vethA, vethB, fmt.Errorf("failed to add link to namespace B: %w", err)
	}

	return vethA, vethB, nil
}

func setupRouting(namespaceA, namespaceB *namespace.NetNS) error {
	// Add default routes for namespaceA and namespaceB via the router
	if err := namespaceA.AddDefaultRoute("10.0.1.1"); err != nil {
		return fmt.Errorf("failed to add route for namespace A: %w", err)
	}

	if err := namespaceB.AddDefaultRoute("10.0.2.1"); err != nil {
		return fmt.Errorf("failed to add route for namespace B: %w", err)
	}

	return nil
}
func setupCleanup(namespaceA, namespaceB, router *namespace.NetNS, vethA, vethB *netlink.Veth) {
	fmt.Println("Cleaning up lab resources...")

	if vethB != nil {
		// This is the back up cleanup, in case the veth pair was not removed when the namespaces were deleted.
		if err := netlink.LinkDel(vethB); err != nil {
			fmt.Printf("Failed to cleanup veth pair: %s\n", err.Error())
		}
	}

	if vethA != nil {
		if err := netlink.LinkDel(vethA); err != nil {
			fmt.Printf("Failed to cleanup veth pair: %s\n", err.Error())
		}
	}

	if router != nil {
		if err := router.Remove(); err != nil {
			fmt.Printf("Failed to remove namespace router: %s\n", err.Error())
		}
	}

	if namespaceB != nil {
		if err := namespaceB.Remove(); err != nil {
			fmt.Printf("Failed to remove namespace host-b: %s\n", err.Error())
		}
	}

	if namespaceA != nil {
		if err := namespaceA.Remove(); err != nil {
			fmt.Printf("Failed to remove namespace host-a: %s\n", err.Error())
		}
	}
}
func Remove() error {
	// Assuming the lab was set up correctly deleting the namespaces should delete the veth pair as well
	err := netns.DeleteNamed("host-a")

	if err != nil {
		return fmt.Errorf("Error deleting namespace host-a: %w", err)
	}
	fmt.Println("Namespace host-a deleted successfully")

	err = netns.DeleteNamed("host-b")
	if err != nil {
		return fmt.Errorf("Error deleting namespace host-b: %w", err)
	}
	fmt.Println("Namespace host-b deleted successfully")

	err = netns.DeleteNamed("router")
	if err != nil {
		return fmt.Errorf("Error deleting namespace router: %w", err)
	}
	fmt.Println("Namespace router deleted successfully")

	fmt.Println("Lab removed successfully")

	return nil
}
