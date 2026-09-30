package lab

import (
	"fmt"
	"runtime"

	"github.com/aniayoub/netlab/internal/namespace"
	"github.com/aniayoub/netlab/internal/vethpair"
	"github.com/vishvananda/netns"
)

func Setup() error {
	// Lock the OS Thread throughout the entire setup process to ensure namespace operations are performed on the correct thread.
	// This should be enhanced later to lock only during critical namespace operations.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var namespaceA, namespaceB *namespace.NetNS
	var veth *vethpair.VethPair
	var err error

	successful_setup := false

	// Store the currentNamespace network namespace so we can switch back to it later.
	currentNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer func() {
		if !successful_setup {
			fmt.Println("Cleaning up lab resources...")

			if namespaceA != nil {
				if err := namespaceA.Remove(); err != nil {
					fmt.Printf("Failed to remove namespace host-a: %s\n", err.Error())
				}
			}
			if namespaceB != nil {
				if err := namespaceB.Remove(); err != nil {
					fmt.Printf("Failed to remove namespace host-b: %s\n", err.Error())
				}
			}
			if veth != nil {
				if err := veth.Cleanup(); err != nil {
					fmt.Printf("Failed to cleanup veth pair: %s\n", err.Error())
				}
			}
		}
	}()

	defer func() {
		fmt.Println("Restoring the original namespace...")
		if err := currentNamespace.Close(); err != nil {
			fmt.Println(fmt.Errorf("failed to close current namespace handle: %w", err))
		}
	}()

	// create namespaces A
	// NOTE: This step automatically switches to the new namespace.
	namespaceA, err = namespace.New("host-a")

	if err != nil {
		return fmt.Errorf("failed to setup namespace host-a: %w", err)
	}

	fmt.Println("Namespace host-a setup successfully")

	// create namespaces B
	// NOTE: This step automatically switches to the new namespace.
	namespaceB, err = namespace.New("host-b")
	if err != nil {
		return fmt.Errorf("failed to setup namespace host-b: %w", err)
	}

	fmt.Println("Namespace host-b setup successfully")

	// Switch back to the original namespace because each namespace creation does automatically switch to the new namespace.
	if err := netns.Set(currentNamespace); err != nil {
		return fmt.Errorf("failed to switch back to the original namespace: %w", err)
	}

	fmt.Println("Switched back to the original namespace successfully")

	// Create veth pair first, we want to operate on each of its endpoints one at a time
	// Otherwise we will need to jump back and forth to different namespaces to configure each endpoint.
	veth, err = vethpair.New(namespaceA.Fd, namespaceB.Fd)
	if err != nil {
		return fmt.Errorf("failed to create veth pair: %w", err)
	}

	fmt.Println("Veth pair created successfully")

	if err := veth.Configure(namespaceB.Fd, vethpair.EndpointB, "10.0.0.2/24"); err != nil {
		return fmt.Errorf("failed to setup veth endpoint B: %w", err)
	}

	if err := veth.Configure(namespaceA.Fd, vethpair.EndpointA, "10.0.0.1/24"); err != nil {
		return fmt.Errorf("failed to setup veth endpoint A: %w", err)
	}
	successful_setup = true

	return nil
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

	fmt.Println("Lab removed successfully")

	return nil
}
