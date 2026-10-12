package integration

import "testing"

func TestLoopbackUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			_, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for _, ns := range namespaces {
				assertLinkUp(t, ns, "lo")
			}
		})
	}
}

func TestLinkInsideTheirNamespace(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for nodeName, node := range nodes {
				for _, iface := range node.Interfaces {
					assertLinkExistsInNamespace(t, nodeName, iface.Name)
				}
			}
		})
	}
}

func TestInterfacesUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for nodeName, node := range nodes {
				for _, iface := range node.Interfaces {
					assertLinkUp(t, nodeName, iface.Name)
				}
			}
		})
	}
}
