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

	"github.com/aniayoub/netlab/internal/config"
)

var netlabBinary string

var fixtures = []struct {
	Name       string
	ConfigPath string
}{
	{
		Name:       "fixture 1: basic topology",
		ConfigPath: "../files/fixture1.yaml",
	},
	{
		Name:       "fixture 2: Host-Router Topology with Multiple Interfaces",
		ConfigPath: "../files/fixture2.yaml",
	},
	{
		Name:       "fixture 3: Dual Router Topology with Transit Link",
		ConfigPath: "../files/fixture3.yaml",
	},
	{
		Name:       "fixture 4: Three Router Topology with Multiple Access Links",
		ConfigPath: "../files/fixture4.yaml",
	},
	{
		Name:       "fixture 5: Four Router Topology with Multiple Access Links",
		ConfigPath: "../files/fixture5.yaml",
	},
}

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
		"../../cmd/netlab",
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
	expectedGateway string,
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

		if route.Gateway != expectedGateway {
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
		"2",
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
func linkExists(t *testing.T, namespace string, iface string) bool {
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

	_, err := cmd.CombinedOutput()

	if err == nil {
		return true
	}

	if _, ok := err.(*exec.ExitError); ok {
		return false
	}

	t.Fatalf(
		"inspect interface %s/%s: %v",
		namespace,
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

func clearLabState(t *testing.T, namespaces []string, ifaces []string) {
	t.Helper()

	for _, namespace := range namespaces {
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
	for _, iface := range ifaces {
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

func runNetlab(t *testing.T, action string, configPath string) string {
	t.Helper()

	cmd := exec.Command(netlabBinary, action, configPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"netlab %s with config %s failed: %v\n%s",
			action,
			configPath,
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

func loadFixture(t *testing.T, path string) (map[string]*config.NodeConfig, []string, []string) {
	cfg, err := config.Parse(path)

	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	namespaces := make([]string, 0, len(cfg.Nodes))
	ifaces := make([]string, 0)
	for _, node := range cfg.Nodes {
		namespaces = append(namespaces, node.Name)
		for _, iface := range node.Interfaces {
			ifaces = append(ifaces, iface.Name)
		}
	}
	return cfg.Nodes, namespaces, ifaces
}
