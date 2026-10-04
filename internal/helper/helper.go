package helper

import (
	"fmt"

	"github.com/vishvananda/netns"
)

func ExecuteInsideNamespace(namespaceFd int, fn func() error) error {
	// Cache the current namespace
	currentNamespace, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get current namespace: %w", err)
	}

	defer func() {
		if err := currentNamespace.Close(); err != nil {
			fmt.Printf("failed to close current namespace handle: %v\n", err)
		}
	}()

	// Set the target namespace
	if err := netns.Set(netns.NsHandle(namespaceFd)); err != nil {
		return fmt.Errorf("failed to set namespace: %w", err)
	}

	defer func() {
		if err := netns.Set(currentNamespace); err != nil {
			fmt.Printf("failed to restore current namespace: %v\n", err)
		}
	}()

	return fn()
}
