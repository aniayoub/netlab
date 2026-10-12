# Netlab

Netlab is a small Linux networking lab tool written in Go. It reads a YAML topology, creates the requested Linux network namespaces and veth links, configures IPv4 addresses and routes, enables forwarding on router nodes, and can remove the resulting lab again from the same configuration.

The project is primarily a networking and Linux systems learning environment. Its goal is to make kernel networking state explicit and inspectable rather than to hide it behind a large orchestration layer.

The current milestone is **Stage 5: configuration-driven topologies**. Earlier stages built the underlying concepts incrementally—network namespaces, ARP and neighbor discovery, Linux bridges, and routed subnets—and remain documented under [`docs/`](docs/).

## What Netlab does today

Given a topology file, `netlab up` can:

- create one network namespace per configured node;
- initialize loopback inside each namespace;
- distinguish `host` and `router` nodes;
- enable IPv4 forwarding inside router namespaces;
- create point-to-point veth pairs for configured links;
- move each veth endpoint into its target namespace;
- rename endpoints to the interface names declared in the topology;
- assign IPv4 CIDR addresses to interfaces;
- rely on Linux to install directly connected routes from those interface addresses;
- install explicitly configured static and default routes;
- support multi-interface hosts, router-to-router transit links, and multi-router topologies;
- roll back resources created by a failed setup attempt while preserving conflicting resources that existed beforehand.

`netlab down` reads the same topology and removes the configured network namespaces. Deleting a namespace also removes the veth endpoints contained by that namespace and therefore their peer links.

Netlab configures Linux networking primitives; it does not implement ARP, IP routing, or packet forwarding itself. Those behaviors remain Linux kernel responsibilities and can be observed with standard tools such as `ip`, `ping`, `traceroute`, and `tcpdump`.

## Requirements

For running Netlab directly:

- Linux
- Go 1.27+
- `iproute2`
- root or equivalent privileges for namespace and link operations

Useful inspection tools include:

- `ping`
- `traceroute`
- `tcpdump`

The implementation uses:

- [`vishvananda/netlink`](https://github.com/vishvananda/netlink) for links, addresses, and routes;
- [`vishvananda/netns`](https://github.com/vishvananda/netns) for Linux network namespaces;
- [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) for topology parsing.

## Build

```bash
go build -o netlab ./cmd/netlab
```

## Usage

Netlab currently exposes two commands and both require a topology file:

```text
netlab up <config-path>
netlab down <config-path>
```

For example, using one of the integration fixtures:

```bash
sudo ./netlab up ./integration/files/fixture3.yaml
```

Inspect the resulting state with Linux tooling:

```bash
ip netns list
ip -n host-a addr
ip -n router-a route
ip -n router-b route
ip -n router-a neigh
```

Test a routed path:

```bash
ip netns exec host-a ping 10.3.5.2
```

Inspect the kernel's route decision:

```bash
ip -n host-a route get 10.3.5.2
ip -n router-a route get 10.3.5.2
```

Remove the topology with the same configuration:

```bash
sudo ./netlab down ./integration/files/fixture3.yaml
```

## Topology model

A topology is composed of **nodes**, **interfaces**, **routes**, and **links**.

A node has a unique name and is either a `host` or a `router`. Each interface declares an IPv4 CIDR. A link connects exactly two endpoints using the `node:interface` form.

Routes are explicit. A route identifies:

- the interface used to reach the next hop;
- a destination CIDR, including `0.0.0.0/0` for a default route;
- the gateway address.

A minimal shape looks like this:

```yaml
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
      - name: access-a
        ip: 10.0.1.1/24

links:
  - endpoints:
      - host-a:eth0
      - router-a:access-a
```

This separation is intentional:

- interface CIDRs produce connected routes automatically through Linux;
- host default gateways are represented as ordinary routes to `0.0.0.0/0`;
- routers use explicit routes for remote subnets;
- router-to-router reachability is described by the topology rather than inferred from link order.

Multiple configured routes on a node are currently assigned increasing route metrics in declaration order.

## Configuration validation

The parser currently rejects several classes of invalid topology before kernel setup begins, including:

- unsupported node types;
- duplicate node names;
- duplicate interface names within a node;
- invalid interface CIDRs;
- malformed link endpoints;
- links with anything other than two endpoints;
- links referencing unknown nodes or interfaces;
- routes referencing unknown interfaces;
- invalid route destination CIDRs;
- invalid route gateway addresses.

Parser behavior is covered by unit tests in [`internal/config/config_test.go`](internal/config/config_test.go).

## Architecture

The Stage 5 refactor moved Netlab away from a fixed lab topology and toward a small configuration-driven orchestration model.

```text
cmd/netlab/
    CLI entry point
    parses the requested topology and dispatches up/down

internal/config/
    YAML DTOs
    topology parsing and validation
    resolved node, interface, route, and link model

internal/lab/
    topology orchestration
    namespace setup
    veth creation and placement
    route installation
    setup rollback
    topology removal

internal/namespace/
    network namespace lifecycle
    namespace-local interface configuration
    static route installation
    IPv4 forwarding

internal/helper/
    executes namespace-sensitive operations in a target network namespace
```

The orchestration layer consumes the resolved configuration model. It no longer assumes fixed namespace names, fixed `/24` networks, one interface per host, or a single router.

## Routing model

Netlab intentionally keeps routing explicit instead of attempting to infer a routing protocol or synthesize remote routes automatically.

For a typical routed path:

```text
host route lookup
    ↓
next-hop router
    ↓
ARP / neighbor resolution on the local link
    ↓
router receives the frame
    ↓
router performs a new route lookup
    ↓
packet is forwarded toward the next subnet or transit router
    ↓
destination host
```

With multiple routers, each router must have routes for remote networks that are not directly connected. Dual-homed hosts can use a primary default route plus more-specific routes through another interface.

The integration fixtures exercise both patterns.

## Integration tests

The integration suite verifies **real Linux kernel state** rather than only inspecting Netlab's internal Go structures. The tests build the `netlab` binary, create actual network namespaces and veth devices, and assert state through commands such as `ip` and `ping`.

The test tree is intentionally separated into reusable topologies and test behavior:

```text
integration/
├── Dockerfile.integration
├── files/
│   ├── config.yaml
│   ├── fixture1.yaml
│   ├── fixture2.yaml
│   ├── fixture3.yaml
│   ├── fixture4.yaml
│   └── fixture5.yaml
└── setup/
    ├── setup_integration_test.go
    ├── lifecycle_integration_test.go
    ├── state_integration_test.go
    ├── routing_integration_test.go
    ├── connectivity_integration_test.go
    └── rollback_integration_test.go
```

### Fixtures

The fixture set grows topology complexity while using the same test harness:

- **Fixture 1** — one router with several directly connected hosts;
- **Fixture 2** — one router with multi-interface / dual-homed hosts and multiple default routes;
- **Fixture 3** — two routers joined by a transit subnet, explicit inter-router routes, and a dual-homed host;
- **Fixture 4** — three routers with multiple access networks and remote static routes;
- **Fixture 5** — four routers and longer multi-hop routed paths.

This lets the integration suite test generalized topology behavior instead of encoding one fixed Stage 4 lab into every test.

### What the suite verifies

The integration suite covers:

- namespace creation and removal;
- loopback initialization;
- interface placement inside the correct namespace;
- administrative link state;
- configured IPv4 addresses;
- configured static and default routes;
- IPv4 forwarding on router nodes;
- representative direct, remote, multi-hop, and dual-homed connectivity paths;
- rollback after namespace conflicts;
- rollback after veth creation conflicts;
- preservation of resources that existed before Netlab attempted setup.

Connectivity expectations are intentionally data-driven per fixture rather than asserting a Cartesian "every node can ping every address" property. Each ping is chosen to prove a meaningful routing behavior.

## Running integration tests

### Recommended: Dockerized environment

The tests are privileged and destructive by nature, so running them in a disposable container is the recommended workflow.

Build the integration image:

```bash
docker build \
  -f ./integration/Dockerfile.integration \
  -t netlab-integration .
```

Run the complete integration suite in a privileged disposable container:

```bash
docker run \
  --rm \
  --privileged \
  netlab-integration \
  go test -tags=integration ./integration/... -v -count=1
```

The container image includes Go plus the Linux networking tools required by the suite. `--privileged` is intentional here because the tests create network namespaces, veth links, routes, and namespace-local forwarding state.

### Running directly on Linux

If you deliberately want to run against the host kernel instead:

```bash
sudo go test -tags=integration ./integration/... -v -count=1
```

Use a disposable or otherwise safe environment when doing this. The suite creates and removes kernel networking resources as part of its assertions and rollback scenarios.

## Current limitations and deferred follow-ups

Stage 5 is considered complete for the current learning milestone, but a few improvements are intentionally deferred:

- validate that each route gateway is reachable through the subnet of its declared interface;
- reject a topology where the same `node:interface` endpoint is attached to more than one link;
- add explicit lifecycle coverage for repeated `down` and repeated `up`, including preservation of an already-running topology after a second `up` fails;
- add `netlab exec <node> -- <command> [args...]`, including unknown-node handling, stdout/stderr forwarding, child exit-status propagation, and integration coverage.

Other current scope boundaries:

- IPv4 only;
- point-to-point veth links only in the current topology model;
- static routing only—no routing protocol or automatic route discovery;
- no persistent Netlab state database;
- no goal of becoming a container runtime, SDN controller, or production network orchestrator.

## Learning stages

The implementation evolved through staged networking exercises:

1. **Stage 1** — network namespaces and direct veth connectivity;
2. **Stage 2** — ARP, neighbor discovery, and packet observation;
3. **Stage 3** — Linux bridges, FDB learning, flooding, and L2 switching;
4. **Stage 4** — routing between subnets, default routes, next-hop ARP, IPv4 forwarding, TTL, and L2 header rewriting;
5. **Stage 5** — remodel the lab as a configuration-driven topology tool with generalized nodes, links, interfaces, routes, rollback, fixtures, and integration coverage.

The earlier notes are still useful as packet-level and kernel-level references even though the current codebase is no longer shaped around those fixed stage topologies.

## Documentation

- [`docs/netlab_learning_plan.md`](docs/netlab_learning_plan.md) — roadmap, stage definitions, project boundaries, and future direction.
- [`docs/stage-0-cheatsheet.md`](docs/stage-0-cheatsheet.md) — Linux network inspection commands.
- [`docs/stage-1-notes.md`](docs/stage-1-notes.md) — namespaces and direct veth connectivity.
- [`docs/stage-2-notes.md`](docs/stage-2-notes.md) — ARP and neighbor resolution.
- [`docs/stage-3-notes.md`](docs/stage-3-notes.md) — bridges, MAC learning, and FDB behavior.
- [`docs/stage-4-notes.md`](docs/stage-4-notes.md) — routing, next hops, forwarding, and packet flow across subnets.

## Repository layout

```text
cmd/netlab/          CLI entry point
internal/config/     YAML topology parsing, validation, and resolved config model
internal/lab/        topology orchestration, route setup, rollback, and removal
internal/namespace/  namespace lifecycle and namespace-local networking
internal/helper/     namespace execution helper
integration/files/   reusable topology fixtures
integration/setup/   kernel-state, routing, connectivity, lifecycle, and rollback tests
integration/         Docker integration-test environment
docs/                roadmap and staged networking notes
```