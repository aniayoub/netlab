//go:build linux && integration

package integration

import "testing"

type pingCase struct {
	Name        string
	Source      string
	Destination string
}

var connectivityCases = map[string][]pingCase{
	"../files/fixture1.yaml": {
		{
			Name:        "host reaches directly connected router",
			Source:      "host-a",
			Destination: "10.1.1.1",
		},
		{
			Name:        "host reaches remote host through router",
			Source:      "host-a",
			Destination: "10.1.5.2",
		},
		{
			Name:        "remote host reaches host in reverse direction",
			Source:      "host-e",
			Destination: "10.1.1.2",
		},
	},

	"../files/fixture2.yaml": {
		{
			Name:        "dual-homed host reaches remote host",
			Source:      "host-a",
			Destination: "10.2.5.2",
		},
		{
			Name:        "second dual-homed host reaches remote host",
			Source:      "host-b",
			Destination: "10.2.5.2",
		},
		{
			Name:        "remote host reaches dual-homed host",
			Source:      "host-c",
			Destination: "10.2.1.2",
		},
	},

	"../files/fixture3.yaml": {
		{
			Name:        "host crosses router transit link",
			Source:      "host-a",
			Destination: "10.3.5.2",
		},
		{
			Name:        "reverse traffic crosses router transit link",
			Source:      "host-d",
			Destination: "10.3.1.2",
		},
		{
			Name:        "router reaches remote host",
			Source:      "router-a",
			Destination: "10.3.5.2",
		},
		{
			Name:        "dual-homed host uses secondary router",
			Source:      "host-c",
			Destination: "10.3.5.2",
		},
		{
			Name:        "dual-homed host reaches network through primary router",
			Source:      "host-c",
			Destination: "10.3.1.2",
		},
	},

	"../files/fixture4.yaml": {
		{
			Name:        "host reaches remote host across three routers",
			Source:      "host-b",
			Destination: "10.4.5.2",
		},
		{
			Name:        "reverse traffic crosses three routers",
			Source:      "host-d",
			Destination: "10.4.3.2",
		},
		{
			Name:        "dual-homed host uses secondary uplink",
			Source:      "host-a",
			Destination: "10.4.5.2",
		},
		{
			Name:        "router reaches host behind remote router",
			Source:      "router-a",
			Destination: "10.4.5.2",
		},
	},

	"../files/fixture5.yaml": {
		{
			Name:        "host reaches remote host across four routers",
			Source:      "host-b",
			Destination: "10.5.13.2",
		},
		{
			Name:        "reverse traffic crosses four routers",
			Source:      "host-j",
			Destination: "10.5.3.2",
		},
		{
			Name:        "dual-homed host uses secondary router",
			Source:      "host-a",
			Destination: "10.5.10.2",
		},
		{
			Name:        "second dual-homed host uses secondary router",
			Source:      "host-e",
			Destination: "10.5.10.2",
		},
		{
			Name:        "router reaches host across remaining router chain",
			Source:      "router-a",
			Destination: "10.5.13.2",
		},
	},
}

func TestConnectivity(t *testing.T) {
	requireIntegrationEnvironment(t)

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			cases, ok := connectivityCases[fixture.ConfigPath]
			if !ok {
				t.Fatalf(
					"no connectivity expectations defined for %s",
					fixture.ConfigPath,
				)
			}

			_, namespaces, ifaces := loadFixture(t, fixture.ConfigPath)

			clearLabState(t, namespaces, ifaces)
			defer clearLabState(t, namespaces, ifaces)

			runNetlab(t, "up", fixture.ConfigPath)

			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					assertPing(t, tc.Source, tc.Destination)
				})
			}
		})
	}
}
