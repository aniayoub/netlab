# Netlab

Netlab is a small programmable Linux networking lab built in Go. It is a learning project for constructing virtual network topologies, inspecting Linux networking state, and reasoning about packet flow from first principles.

The project is intentionally scoped around Linux networking primitives rather than building a production router, container runtime, SDN controller, or TCP stack. The full roadmap and project boundaries are documented in [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md).

## Current status

Stage 3 is implemented: two isolated virtual hosts connected through a Linux bridge acting as a Layer-2 switch.

```text
host-a namespace                         root namespace                         host-b namespace

10.0.0.1/24                                                                       10.0.0.2/24
    eth0                                                                              eth0
      |                                                                                |
      | veth                                                                           | veth
      |                                                                                |
   port-a ---------------------------------- br0 ----------------------------------- port-b
```

`netlab up` currently:

- creates the `host-a` and `host-b` network namespaces;
- brings loopback up inside each namespace;
- creates the `br0` Linux bridge and brings it up;
- creates one veth pair per host;
- keeps `port-a` and `port-b` in the root namespace and attaches them to `br0`;
- moves the host-side endpoint of each pair into its namespace as `eth0`;
- assigns `10.0.0.1/24` and `10.0.0.2/24` to the host interfaces;
- brings the host interfaces and bridge ports up;
- relies on Linux to create the connected `10.0.0.0/24` routes;
- rolls back resources created during setup if setup fails partway through.

`netlab down` removes the named namespaces and the bridge. Deleting the namespaces removes their veth endpoints, which also removes the corresponding root-side veth peers.

The next stage introduces routing between different IPv4 subnets.

## Requirements

- Linux
- Go 1.27+
- `iproute2` (`ip` and `bridge`)
- `ping`
- `tcpdump` for packet-flow experiments
- root or equivalent privileges for network namespace and link operations

The implementation uses [`vishvananda/netlink`](https://github.com/vishvananda/netlink) and [`vishvananda/netns`](https://github.com/vishvananda/netns) to interact with Linux networking directly.

## Build

```bash
go build -o netlab ./cmd/netlab
```

## Usage

Create the lab:

```bash
sudo ./netlab up
```

Inspect the topology with standard Linux tools:

```bash
ip netns list
ip -n host-a addr
ip -n host-a route
ip link show master br0
bridge fdb show br br0
ip netns exec host-a ping 10.0.0.2
```

Remove the lab:

```bash
sudo ./netlab down
```

## Integration tests

The integration suite builds and exercises the `netlab` binary and verifies the resulting kernel state using standard Linux tooling.

```bash
sudo go test -tags=integration ./integration -v -count=1
```

The suite verifies, among other things:

- namespace creation and removal;
- loopback and host-interface state;
- IPv4 addressing and connected routes;
- creation and state of `br0`, `port-a`, and `port-b`;
- bridge membership of both ports;
- bidirectional host connectivity;
- rollback behavior when setup encounters resource conflicts.

The integration tests are destructive and assume an isolated or disposable Linux environment. They use fixed resource names including `host-a`, `host-b`, `br0`, `port-a`, and `port-b`, and may remove matching resources during test cleanup.

## Current implementation constraint

The host-side veth endpoint is currently created as `eth0` in the root namespace before being moved into the target namespace. As a result, setup assumes that `eth0` is available in the root namespace during veth creation.

This is an accepted limitation for the current learning stage rather than a production-ready naming strategy.

## Documentation

- [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md) — project roadmap, stage definitions, MVP boundary, and stretch goals.
- [`docs/stage-0-cheatsheet.md`](docs/stage-0-cheatsheet.md) — Linux network inspection reference for interfaces, addresses, routes, neighbors, and sockets.
- [`docs/stage-1-notes.md`](docs/stage-1-notes.md) — observations from implementing network namespaces and direct veth connectivity.
- [`docs/stage-2-notes.md`](docs/stage-2-notes.md) — ARP, neighbor resolution, and packet-flow observations.
- [`docs/stage-3-notes.md`](docs/stage-3-notes.md) — Linux bridge construction, MAC learning, FDB behavior, and Stage 3 lifecycle decisions.

## Repository layout

```text
cmd/netlab/          CLI entry point
internal/lab/        lab lifecycle orchestration and rollback
internal/namespace/  network namespace setup
internal/vethpair/   veth creation, bridge-port attachment, and host-interface configuration
integration/         end-to-end integration tests
docs/                roadmap and learning notes
```