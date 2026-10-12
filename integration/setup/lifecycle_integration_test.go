package integration

import "testing"

func TestTopologyUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)
			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for _, node := range nodes {
				assertNamespaceExists(t, node.Name)

				for _, iface := range node.Interfaces {
					assertLinkExistsInNamespace(t, node.Name, iface.Name)

					assertRootLinkAbsent(t, iface.Name)
				}
			}
		})
	}
}

func TestTopologyDown(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)
			runNetlab(t, "down", fixture.ConfigPath)

			// We assume the netlab is successfully brought up, so all interfaces should be absent if their namespaces are deleted.
			for _, node := range nodes {
				assertNamespaceAbsent(t, node.Name)
				for _, iface := range node.Interfaces {
					assertRootLinkAbsent(t, iface.Name)
				}
			}
		})
	}
}
