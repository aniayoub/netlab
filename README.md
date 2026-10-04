# Netlab

Netlab is a small programmable Linux networking lab built in Go. It is a learning project for constructing virtual network topologies, inspecting Linux networking state, and reasoning about packet flow from first principles.

The project is intentionally scoped around Linux networking primitives rather than building a production router, container runtime, SDN controller, or TCP stack. The full roadmap and project boundaries are documented in [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md).

## Current status

Stage 4 is implemented: two isolated hosts on different IPv4 subnets communicate through a Linux network namespace acting as a router.

```text
10.0.1.0/24                              10.0.2.0/24

host-a namespace        router namespace        host-b namespace

10.0.1.2/24        10.0.1.1/24  10.0.2.1/24       10.0.2.2/24
    eth0  <-------->  port-a        port-b  <-------->  eth0

 default via                                      default via
 10.0.1.1                                         10.0.2.1
```

`netlab up` currently:

- creates the `host-a`, `router`, and `host-b` network namespaces;
- brings loopback up while each namespace is initialized;
- enables IPv4 forwarding inside the `router` namespace;
- creates two veth pairs: `eth-a <-> port-a` and `eth-b <-> port-b`;
- moves `port-a` and `port-b` into the router namespace;
- moves `eth-a` and `eth-b` into the host namespaces and renames each host interface to `eth0`;
- assigns `10.0.1.2/24` to `host-a/eth0` and `10.0.1.1/24` to `router/port-a`;
- assigns `10.0.2.2/24` to `host-b/eth0` and `10.0.2.1/24` to `router/port-b`;
- relies on Linux to create the connected routes for `10.0.1.0/24` and `10.0.2.0/24`;
- installs a default route on each host through its directly connected router interface;
- rolls back resources created by the current setup attempt if setup fails partway through.

`netlab down` deletes the three named namespaces. Removing the namespaces also removes the veth endpoints they contain and therefore their peers.

The resulting lab is deliberately small enough to inspect with standard Linux tooling while exercising routing tables, next hops, ARP/neighbor resolution, IPv4 forwarding, and routed packet flow.

## Requirements

- Linux
- Go 1.27+
- `iproute2` (`ip`)
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

Inspect namespace and interface state:

```bash
ip netns list
ip -n host-a addr
ip -n router addr
ip -n host-b addr
```

Inspect routing decisions:

```bash
ip -n host-a route
ip -n router route
ip -n host-b route

ip -n host-a route get 10.0.2.2
ip -n router route get 10.0.2.2
```

Inspect neighbor resolution:

```bash
ip -n host-a neigh
ip -n router neigh
```

Verify router forwarding:

```bash
ip netns exec router cat /proc/sys/net/ipv4/ip_forward
```

Test cross-subnet connectivity:

```bash
ip netns exec host-a ping 10.0.2.2
ip netns exec host-b ping 10.0.1.2
```

Observe routed traffic:

```bash
ip netns exec router tcpdump -n -e -i port-a 'arp or icmp'
ip netns exec router tcpdump -n -e -i port-b 'arp or icmp'
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

- creation and removal of `host-a`, `router`, and `host-b`;
- host and router interface placement;
- administrative interface state;
- IPv4 addressing on all routed interfaces;
- connected routes for both IPv4 subnets;
- host default routes through the router;
- IPv4 forwarding inside the router namespace;
- bidirectional cross-subnet connectivity;
- rollback when setup encounters namespace or veth resource conflicts;
- preservation of conflicting resources that were not created by Netlab.

The connectivity assertions send multiple ICMP requests rather than requiring the very first packet after setup to succeed. Kernel link state and neighbor discovery can require a short convergence period even after the configuration operations have returned successfully.

The integration tests are destructive and assume an isolated or disposable Linux environment. They use fixed names including `host-a`, `host-b`, `router`, `eth-a`, `eth-b`, `port-a`, and `port-b`, and cleanup may remove matching resources created by the test lab.

## Current architecture

The Stage 4 implementation uses a deliberately small internal model.

```text
cmd/netlab/
    CLI entry point

internal/lab/
    topology orchestration
    veth creation
    routing setup
    setup rollback

internal/namespace/
    network namespace lifecycle
    namespace-local link configuration
    default routes
    IPv4 forwarding

internal/helper/
    execute operations inside a target network namespace

integration/
    end-to-end kernel-state and connectivity tests
```

The application configures Linux networking primitives; it does not implement switching, ARP, routing, or packet forwarding itself.

## Networking model demonstrated by Stage 4

For traffic from `host-a` to `host-b`:

```text
10.0.2.2 is not on host-a's local subnet
        ↓
host-a selects its default route
        ↓
next hop = 10.0.1.1
        ↓
ARP resolves the router's local MAC address
        ↓
frame reaches router/port-a
        ↓
router performs a new route lookup
        ↓
10.0.2.0/24 is directly connected through port-b
        ↓
router forwards the packet
        ↓
host-b receives traffic for 10.0.2.2
```

Across the router, the Ethernet source and destination addresses change for the new link while the original IP source and destination remain associated with the end hosts. The router also decrements the IPv4 TTL when forwarding the packet.

See [`docs/stage-4-notes.md`](docs/stage-4-notes.md) for the full Stage 4 observations.

## Documentation

- [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md) — project roadmap, stage definitions, MVP boundary, and stretch goals.
- [`docs/stage-0-cheatsheet.md`](docs/stage-0-cheatsheet.md) — Linux network inspection reference for interfaces, addresses, routes, neighbors, and sockets.
- [`docs/stage-1-notes.md`](docs/stage-1-notes.md) — observations from implementing network namespaces and direct veth connectivity.
- [`docs/stage-2-notes.md`](docs/stage-2-notes.md) — ARP, neighbor resolution, and packet-flow observations.
- [`docs/stage-3-notes.md`](docs/stage-3-notes.md) — Linux bridge construction, MAC learning, FDB behavior, and lifecycle decisions.
- [`docs/stage-4-notes.md`](docs/stage-4-notes.md) — routing tables, default gateways, next-hop resolution, IPv4 forwarding, and routed packet flow.

## Repository layout

```text
cmd/netlab/          CLI entry point
internal/lab/        topology orchestration and rollback
internal/namespace/  namespace lifecycle and namespace-local networking
internal/helper/     namespace execution helper
integration/         end-to-end integration tests
docs/                roadmap and learning notes
```