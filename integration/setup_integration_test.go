//go:build linux && integration

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	HostA = "host-a"
	HostB = "host-b"

	Bridge = "br0"
	Eth0   = "eth0"
	PortA  = "port-a"
	PortB  = "port-b"
	/*VethA = "veth0"
	VethB = "veth1"
	*/
	AddrA = "10.0.0.1"
	AddrB = "10.0.0.2"

	PrefixLen = 24
	Subnet    = "10.0.0.0/24"
)

var netlabBinary string

type linkInfo struct {
	IfName    string   `json:"ifname"`
	Flags     []string `json:"flags"`
	OperState string   `json:"operstate"`
	Master    string   `json:"master"`
}

type addrInfo struct {
	IfName   string `json:"ifname"`
	AddrInfo []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		PrefixLen int    `json:"prefixlen"`
	} `json:"addr_info"`
}

type routeInfo struct {
	Dst     string `json:"dst"`
	Dev     string `json:"dev"`
	Gateway string `json:"gateway"`
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "netlab-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create integration test temp dir: %v\n", err)
		os.Exit(1)
	}

	netlabBinary = filepath.Join(dir, "netlab")

	cmd := exec.Command(
		"go",
		"build",
		"-o",
		netlabBinary,
		"../cmd/netlab",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"build netlab: %v\n%s\n",
			err,
			output,
		)

		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// Integration tests are intentionally destructive.
//
// They assume an isolated/disposable Linux environment and may delete
// namespaces and root links using the fixed Stage 1 resource names.

// -----------------------------------------------------------------------------
// Lab lifecycle
// -----------------------------------------------------------------------------

func TestHostUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertNamespaceExists(t, HostA)
	assertNamespaceExists(t, HostB)

	assertRootLinkExists(t, Bridge)
	assertRootLinkExists(t, PortA)
	assertRootLinkExists(t, PortB)

}

func TestHostDown(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	// To test the host down functionality, we first bring the lab up.
	// However, this assumes that the lab can be brought down only if it was previously brought up.
	// The question remains, should we have the case where the lab should bring down (clear namespaces and their associated dependencies) even if it was setup outside netlab!
	runNetlab(t, "up")
	runNetlab(t, "down")

	assertNamespaceAbsent(t, HostA)
	assertNamespaceAbsent(t, HostB)

	assertRootLinkAbsent(t, Bridge)

	assertRootLinkAbsent(t, PortA)
	assertRootLinkAbsent(t, PortB)
}

// -----------------------------------------------------------------------------
// Resource conflicts / rollback
// -----------------------------------------------------------------------------

func TestExistingHostUpConflict(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	// Simulate a resource that existed before netlab setup.
	runIP(t, "netns", "add", HostA)

	output, err := exec.Command(netlabBinary, "up").CombinedOutput()
	if err == nil {
		t.Fatalf(
			"expected netlab up to fail with existing %s\n%s",
			HostA,
			output,
		)
	}

	// netlab must not delete a resource it did not create.
	assertNamespaceExists(t, HostA)

	// net lab should not create Host B, since the sequence is create host A first, then host B.
	// However, this is not the best way to test this behaviour as it relies that the order is guanranteed by the implementation.
	assertNamespaceAbsent(t, HostB)

	assertRootLinkAbsent(t, Bridge)
	assertRootLinkAbsent(t, PortA)
	assertRootLinkAbsent(t, PortB)
}

func TestNamespaceCreationFailRollsBack(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	// host-a should be created successfully first, then creation of host-b
	// should fail because this fixture already owns it.
	runIP(t, "netns", "add", HostB)

	output, err := exec.Command(netlabBinary, "up").CombinedOutput()
	if err == nil {
		t.Fatalf(
			"expected netlab up to fail with existing %s\n%s",
			HostB,
			output,
		)
	}

	// host-a was created by this setup attempt and must be rolled back.
	assertNamespaceAbsent(t, HostA)

	// host-b existed beforehand and must survive.
	assertNamespaceExists(t, HostB)

	assertRootLinkAbsent(t, Bridge)
	assertRootLinkAbsent(t, PortA)
	assertRootLinkAbsent(t, PortB)
}

func TestBridgeCreationFailRollsBack(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	// Simulate a veth-name collision owned by the test fixture.
	runIP(
		t,
		"link",
		"add",
		PortB,
		"type",
		"dummy",
	)

	output, err := exec.Command(netlabBinary, "up").CombinedOutput()
	if err == nil {
		t.Fatalf(
			"expected netlab up to fail with existing bridge\n%s",
			output,
		)
	}

	// Both namespaces were created by this setup attempt and must be rolled back.
	assertNamespaceAbsent(t, HostA)
	assertNamespaceAbsent(t, HostB)

	assertRootLinkAbsent(t, Bridge)
	assertRootLinkAbsent(t, PortA)

	assertRootLinkExists(t, PortB)
}

// -----------------------------------------------------------------------------
// Namespace / interface state
// -----------------------------------------------------------------------------

func TestLoopbackUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertLinkUp(t, HostA, "lo")
	assertLinkUp(t, HostB, "lo")
}

func TestEthInsideNamespace(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertLinkExistsInNamespace(t, HostA, Eth0)
	assertLinkExistsInNamespace(t, HostB, Eth0)

	// Once moved, neither endpoint should remain visible in the root namespace.
	assertRootLinkAbsent(t, Eth0)
}

func TestHostEthUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertLinkUp(t, HostA, Eth0)
	assertLinkUp(t, HostB, Eth0)
}

func TestBridgeExists(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertRootLinkExists(t, Bridge)
	assertRootLinkExists(t, PortA)
	assertRootLinkExists(t, PortB)
}

func TestBridgeUp(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertRootLinkUp(t, Bridge)
	assertRootLinkUp(t, PortA)
	assertRootLinkUp(t, PortB)
}

func TestBridgeMaster(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertBridgeMaster(t, PortA, Bridge)
	assertBridgeMaster(t, PortB, Bridge)
}

// -----------------------------------------------------------------------------
// Addressing / routing
// -----------------------------------------------------------------------------

func TestEthAddress(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertIPv4Address(t, HostA, Eth0, AddrA, PrefixLen)
	assertIPv4Address(t, HostB, Eth0, AddrB, PrefixLen)
}

func TestConnectedRoutes(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertConnectedRoute(t, HostA, Subnet, Eth0)
	assertConnectedRoute(t, HostB, Subnet, Eth0)
}

// -----------------------------------------------------------------------------
// Connectivity
// -----------------------------------------------------------------------------

func TestHostAPingsHostB(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertPing(t, HostA, AddrB)
}

func TestHostBPingsHostA(t *testing.T) {
	requireIntegrationEnvironment(t)

	clearLabState(t)
	defer clearLabState(t)

	runNetlab(t, "up")

	assertPing(t, HostB, AddrA)
}

// -----------------------------------------------------------------------------
// Assertions
// -----------------------------------------------------------------------------

func assertNamespaceExists(t *testing.T, namespace string) {
	t.Helper()

	if !namespaceExists(t, namespace) {
		t.Fatalf("namespace %q does not exist", namespace)
	}
}

func assertNamespaceAbsent(t *testing.T, namespace string) {
	t.Helper()

	if namespaceExists(t, namespace) {
		t.Fatalf("namespace %q unexpectedly exists", namespace)
	}
}

func assertLinkExistsInNamespace(
	t *testing.T,
	namespace string,
	iface string,
) {
	t.Helper()

	cmd := exec.Command(
		"ip",
		"-n",
		namespace,
		"link",
		"show",
		"dev",
		iface,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"interface %s does not exist in namespace %s: %v\n%s",
			iface,
			namespace,
			err,
			output,
		)
	}
}

func assertLinkUp(
	t *testing.T,
	namespace string,
	iface string,
) {
	t.Helper()

	link := getLink(t, namespace, iface)

	if !contains(link.Flags, "UP") {
		t.Fatalf(
			"interface %s in namespace %s is not administratively UP: flags=%v operstate=%s",
			iface,
			namespace,
			link.Flags,
			link.OperState,
		)
	}
}

func assertIPv4Address(
	t *testing.T,
	namespace string,
	iface string,
	expectedAddress string,
	expectedPrefix int,
) {
	t.Helper()

	output := runIP(
		t,
		"-j",
		"-n",
		namespace,
		"addr",
		"show",
		"dev",
		iface,
	)

	var links []addrInfo

	if err := json.Unmarshal([]byte(output), &links); err != nil {
		t.Fatalf(
			"parse address state for %s/%s: %v\n%s",
			namespace,
			iface,
			err,
			output,
		)
	}

	for _, link := range links {
		for _, addr := range link.AddrInfo {
			if addr.Family != "inet" {
				continue
			}

			if addr.Local == expectedAddress &&
				addr.PrefixLen == expectedPrefix {
				return
			}
		}
	}

	t.Fatalf(
		"%s/%s does not have expected IPv4 address %s/%d\n%s",
		namespace,
		iface,
		expectedAddress,
		expectedPrefix,
		output,
	)
}

func assertConnectedRoute(
	t *testing.T,
	namespace string,
	expectedSubnet string,
	expectedInterface string,
) {
	t.Helper()

	output := runIP(
		t,
		"-j",
		"-n",
		namespace,
		"route",
		"show",
		expectedSubnet,
	)

	var routes []routeInfo

	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		t.Fatalf(
			"parse route state in namespace %s: %v\n%s",
			namespace,
			err,
			output,
		)
	}

	for _, route := range routes {
		if route.Dst != expectedSubnet {
			continue
		}

		if route.Dev != expectedInterface {
			continue
		}

		if route.Gateway != "" {
			t.Fatalf(
				"route %s in namespace %s unexpectedly uses gateway %s",
				expectedSubnet,
				namespace,
				route.Gateway,
			)
		}

		return
	}

	t.Fatalf(
		"connected route %s dev %s not found in namespace %s\n%s",
		expectedSubnet,
		expectedInterface,
		namespace,
		output,
	)
}

func assertPing(
	t *testing.T,
	sourceNamespace string,
	destination string,
) {
	t.Helper()

	cmd := exec.Command(
		"ip",
		"netns",
		"exec",
		sourceNamespace,
		"ping",
		"-c",
		"3",
		"-W",
		"1",
		destination,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"ping from %s to %s failed: %v\n%s",
			sourceNamespace,
			destination,
			err,
			output,
		)
	}
}

func assertRootLinkExists(t *testing.T, iface string) {
	t.Helper()

	if !rootLinkExists(t, iface) {
		t.Fatalf("root interface %q does not exist", iface)
	}
}

func assertRootLinkAbsent(t *testing.T, iface string) {
	t.Helper()

	if rootLinkExists(t, iface) {
		t.Fatalf("root interface %q unexpectedly exists", iface)
	}
}
func assertRootLinkUp(t *testing.T, iface string) {
	t.Helper()

	link := runIP(
		t,
		"-j",
		"link",
		"show",
		"dev",
		iface,
	)

	var links []linkInfo

	if err := json.Unmarshal([]byte(link), &links); err != nil {
		t.Fatalf(
			"parse link state for root interface %s: %v\n%s",
			iface,
			err,
			link,
		)
	}

	if len(links) != 1 {
		t.Fatalf(
			"expected one root interface %s, got %d",
			iface,
			len(links),
		)
	}
	if !contains(links[0].Flags, "UP") {
		t.Fatalf(
			"interface %s in root is not administratively UP: flags=%v operstate=%s",
			iface,
			links[0].Flags,
			links[0].OperState,
		)
	}
}

func assertBridgeMaster(t *testing.T, portIface string, masterIface string) {
	t.Helper()

	link := runIP(
		t,
		"-j",
		"link",
		"show",
		"dev",
		portIface,
	)

	var links []linkInfo

	if err := json.Unmarshal([]byte(link), &links); err != nil {
		t.Fatalf(
			"parse link state for bridge interface %s: %v\n%s",
			portIface,
			err,
			link,
		)
	}

	if len(links) != 1 {
		t.Fatalf(
			"expected one bridge interface %s, got %d",
			portIface,
			len(links),
		)
	}

	if links[0].Master != masterIface {
		t.Fatalf(
			"bridge interface %s is not enslaved to master %s: master=%s",
			portIface,
			masterIface,
			links[0].Master,
		)
	}
}

// -----------------------------------------------------------------------------
// Inspection helpers
// -----------------------------------------------------------------------------

func getLink(
	t *testing.T,
	namespace string,
	iface string,
) linkInfo {
	t.Helper()

	output := runIP(
		t,
		"-j",
		"-n",
		namespace,
		"link",
		"show",
		"dev",
		iface,
	)

	var links []linkInfo

	if err := json.Unmarshal([]byte(output), &links); err != nil {
		t.Fatalf(
			"parse link state for %s/%s: %v\n%s",
			namespace,
			iface,
			err,
			output,
		)
	}

	if len(links) != 1 {
		t.Fatalf(
			"expected one interface for %s/%s, got %d",
			namespace,
			iface,
			len(links),
		)
	}

	return links[0]
}

func namespaceExists(t *testing.T, namespace string) bool {
	t.Helper()

	_, err := os.Stat("/var/run/netns/" + namespace)

	switch {
	case err == nil:
		return true
	case os.IsNotExist(err):
		return false
	default:
		t.Fatalf(
			"inspect namespace %s: %v",
			namespace,
			err,
		)

		return false
	}
}

func rootLinkExists(t *testing.T, iface string) bool {
	t.Helper()

	cmd := exec.Command(
		"ip",
		"link",
		"show",
		"dev",
		iface,
	)

	_, err := cmd.CombinedOutput()

	if err == nil {
		return true
	}

	if _, ok := err.(*exec.ExitError); ok {
		return false
	}

	t.Fatalf(
		"inspect root interface %s: %v",
		iface,
		err,
	)

	return false
}

// -----------------------------------------------------------------------------
// Test environment
// -----------------------------------------------------------------------------

func requireIntegrationEnvironment(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("integration test requires root")
	}

	for _, tool := range []string{"ip", "ping"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf(
				"integration test requires %s",
				tool,
			)
		}
	}
}

func clearLabState(t *testing.T) {
	t.Helper()

	// Namespaces first. If they contain the Stage 1 veth endpoints,
	// deleting the namespaces removes those endpoints with them.
	for _, namespace := range []string{HostA, HostB} {
		if !namespaceExists(t, namespace) {
			continue
		}

		runIP(
			t,
			"netns",
			"delete",
			namespace,
		)
	}

	// Remove root-level remnants or deliberate collision fixtures.
	// Deleting one endpoint of a veth pair also deletes its peer.
	for _, iface := range []string{Bridge, PortA, PortB} {
		if !rootLinkExists(t, iface) {
			continue
		}

		// We handle deletion without runIP because the veth pair might already be removed
		// when the namespace was deleted. Attempting to delete it again could result in an error.
		cmd := exec.Command("ip", "link", "delete", iface)
		_, err := cmd.CombinedOutput()
		if err != nil {
			if !rootLinkExists(t, iface) {
				t.Logf("Link %s does not exist, nothing to clean up\n", iface)
				continue
			}
			t.Fatalf("Failed to delete link %s: %v\n", iface, err)
		}
	}
}

func runNetlab(t *testing.T, args ...string) string {
	t.Helper()

	cmd := exec.Command(netlabBinary, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"netlab %s failed: %v\n%s",
			strings.Join(args, " "),
			err,
			output,
		)
	}

	return string(output)
}

func runIP(t *testing.T, args ...string) string {
	t.Helper()

	cmd := exec.Command("ip", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"ip %s failed: %v\n%s",
			strings.Join(args, " "),
			err,
			output,
		)
	}

	return string(output)
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}

	return false
}
