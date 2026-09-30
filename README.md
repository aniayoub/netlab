# Netlab

Netlab is a small programmable Linux networking lab built in Go. It is a learning project for constructing virtual network topologies, inspecting Linux networking state, and reasoning about packet flow from first principles.

The project is intentionally scoped around Linux networking primitives rather than building a production router, container runtime, SDN controller, or TCP stack. The full roadmap and project boundaries are documented in [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md).

## Current status

Stage 1 is implemented: two isolated virtual hosts connected by a veth pair.

```text
host-a                              host-b
10.0.0.1/24                        10.0.0.2/24
    |                                  |
  veth0  <======================>   veth1
```

`netlab up` currently:

- creates the `host-a` and `host-b` network namespaces;
- creates a veth pair and moves one endpoint into each namespace;
- assigns `10.0.0.1/24` and `10.0.0.2/24`;
- brings the loopback and veth interfaces up;
- relies on Linux to create the connected `10.0.0.0/24` routes.

`netlab down` removes the lab namespaces and their associated network resources.

The next stage focuses on observing ARP and ICMP packet flow over this topology.

## Requirements

- Linux
- Go 1.27+
- `iproute2`
- `ping`
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

Inspect it with standard Linux tools:

```bash
ip netns list
ip -n host-a addr
ip -n host-a route
ip netns exec host-a ping 10.0.0.2
```

Remove the lab:

```bash
sudo ./netlab down
```

## Integration tests

The integration suite builds and exercises the `netlab` binary and verifies the resulting kernel state using `ip` and `ping`.

```bash
sudo go test -tags=integration ./integration -v -count=1
```

The integration tests are destructive and assume an isolated or disposable Linux environment. They may remove namespaces and links using the fixed Stage 1 resource names (`host-a`, `host-b`, `veth0`, and `veth1`).

## Documentation

- [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md) — project roadmap, stage definitions, MVP boundary, and stretch goals.
- [`docs/stage-0-cheatsheet.md`](docs/stage-0-cheatsheet.md) — Linux network inspection reference for interfaces, addresses, routes, neighbors, and sockets.
- [`docs/stage-1-notes.md`](docs/stage-1-notes.md) — observations from implementing network namespaces and veth-based connectivity.

## Repository layout

```text
cmd/netlab/          CLI entry point
internal/lab/        lab lifecycle orchestration
internal/namespace/  network namespace setup
internal/vethpair/   veth creation and configuration
integration/         end-to-end integration tests
docs/                roadmap and learning notes
```
