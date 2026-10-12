package integration

import (
	"strings"
	"testing"

	"github.com/aniayoub/netlab/internal/config"
)

func TestAddresses(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for nodeName, node := range nodes {
				for _, iface := range node.Interfaces {
					prefixSize, _ := iface.IpNet.Mask.Size()
					assertIPv4Address(t, nodeName, iface.Name, iface.IpNet.IP.String(), prefixSize)
				}
			}
		})
	}
}

func TestConnectedRoutes(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for _, node := range nodes {
				for _, route := range node.Routes {
					dst := route.Destination
					if dst == "0.0.0.0/0" {
						dst = "default"
					}
					assertConnectedRoute(t, node.Name, dst, route.Gateway, route.Interface)
				}
			}
		})
	}
}

func TestIpv4Forwarding(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			nodes, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for nodeName, node := range nodes {
				if node.Type == config.Router {
					output := runIP(t,
						"netns",
						"exec",
						nodeName,
						"cat",
						"/proc/sys/net/ipv4/ip_forward",
					)

					if strings.TrimSpace(output) != "1" {
						t.Fatalf("IPv4 forwarding is not enabled in namespace %s: got %q", nodeName, output)
					}
				}
			}
		})
	}

}
