package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	path := writeConfig(t, `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24
    routes:
      - interface: eth0
        destination: 0.0.0.0/0
        gateway: 10.0.1.1

  - name: router-a
    type: router
    interfaces:
      - name: eth0
        ip: 10.0.1.1/24

links:
  - endpoints:
      - host-a:eth0
      - router-a:eth0
`)

	cfg, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(cfg.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(cfg.Nodes))
	}

	host := cfg.Nodes["host-a"]
	if host == nil {
		t.Fatal("expected host-a to exist")
	}

	if host.Type != Host {
		t.Errorf("expected host-a type %q, got %q", Host, host.Type)
	}

	iface := host.Interfaces["eth0"]
	if iface == nil {
		t.Fatal("expected host-a:eth0 to exist")
	}

	if got := iface.IpNet.String(); got != "10.0.1.2/24" {
		t.Errorf("expected IP 10.0.1.2/24, got %s", got)
	}

	if len(host.Routes) != 1 {
		t.Fatalf("expected 1 host route, got %d", len(host.Routes))
	}

	route := host.Routes[0]

	if route.Interface != "eth0" {
		t.Errorf("expected route interface eth0, got %q", route.Interface)
	}

	if route.Destination != "0.0.0.0/0" {
		t.Errorf("expected default route, got %q", route.Destination)
	}

	if route.Gateway != "10.0.1.1" {
		t.Errorf("expected gateway 10.0.1.1, got %q", route.Gateway)
	}

	if len(cfg.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(cfg.Links))
	}

	link := cfg.Links[0]

	if link.SrcNode != "host-a" ||
		link.SrcInterface != "eth0" ||
		link.DstNode != "router-a" ||
		link.DstInterface != "eth0" {
		t.Errorf("unexpected parsed link: %+v", link)
	}
}

func TestParseValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantErrText string
	}{
		{
			name: "malformed yaml",
			yaml: `
nodes: [
`,
			wantErrText: "did not find expected node content",
		},
		{
			name: "duplicate node",
			yaml: `
nodes:
  - name: host-a
    type: host

  - name: host-a
    type: host
`,
			wantErrText: "duplicate node name found: host-a",
		},
		{
			name: "duplicate interface",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24
      - name: eth0
        ip: 10.0.2.2/24
`,
			wantErrText: `duplicate interface name "eth0" on node "host-a"`,
		},
		{
			name: "invalid interface cidr",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: not-a-cidr
`,
			wantErrText: `invalid CIDR "not-a-cidr"`,
		},
		{
			name: "link has wrong number of endpoints",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24

links:
  - endpoints:
      - host-a:eth0
`,
			wantErrText: "link must have exactly 2 endpoints",
		},
		{
			name: "invalid endpoint format",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24

links:
  - endpoints:
      - host-a
      - host-a:eth0
`,
			wantErrText: `invalid endpoint format "host-a"`,
		},
		{
			name: "link references unknown node",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24

links:
  - endpoints:
      - host-a:eth0
      - router-a:eth0
`,
			wantErrText: "link references non-existent node: router-a",
		},
		{
			name: "link references unknown interface",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24

  - name: router-a
    type: router
    interfaces:
      - name: eth0
        ip: 10.0.1.1/24

links:
  - endpoints:
      - host-a:eth1
      - router-a:eth0
`,
			wantErrText: `link references non-existent interface "eth1" on node "host-a"`,
		},
		{
			name: "route references unknown interface",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24
    routes:
      - interface: eth1
        destination: 0.0.0.0/0
        gateway: 10.0.1.1
`,
			wantErrText: `no interface found on node "host-a"`,
		},
		{
			name: "invalid route destination",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24
    routes:
      - interface: eth0
        destination: invalid
        gateway: 10.0.1.1
`,
			wantErrText: `invalid CIDR "invalid" on route of node "host-a"`,
		},
		{
			name: "invalid route gateway",
			yaml: `
nodes:
  - name: host-a
    type: host
    interfaces:
      - name: eth0
        ip: 10.0.1.2/24
    routes:
      - interface: eth0
        destination: 0.0.0.0/0
        gateway: invalid
`,
			wantErrText: `invalid IP "invalid" on route of node "host-a"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.yaml)

			_, err := Parse(path)
			if err == nil {
				t.Fatal("Parse() expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf(
					"Parse() error = %q, expected it to contain %q",
					err,
					tt.wantErrText,
				)
			}
		})
	}
}

func TestParseFileDoesNotExist(t *testing.T) {
	_, err := Parse(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("Parse() expected error for missing file")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return path
}
