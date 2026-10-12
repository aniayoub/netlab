# Stage 5 Notes — Configuration-Driven Topologies

## Goal

Stage 5 remodels Netlab from a fixed networking exercise into a small topology-driven Linux networking tool.

A YAML file now describes:

- nodes;
- node roles;
- interfaces;
- IPv4 addresses;
- links;
- static routes.

`netlab up <config>` creates the topology and `netlab down <config>` removes it.

## Main design

The configuration file is the desired topology.

Netlab translates that into Linux kernel resources:

```text
YAML topology
    ↓
validated config
    ↓
network namespaces
    ↓
veth links
    ↓
addresses
    ↓
routes
    ↓
kernel forwarding
```

Connected routes are created automatically by Linux when interface addresses are assigned.

All non-connected routes are explicit in the topology, including default routes.

## Routing rules

Hosts typically use:

```text
0.0.0.0/0 → local router
```

Routers use destination-specific routes for remote networks.

For multi-router topologies:

```text
host
  ↓ default route
router A
  ↓ static route
router B
  ↓ connected route
destination
```

Dual-homed hosts can use one default route and more-specific routes through the secondary interface.

## Architecture

Stage 5 separates responsibilities into small packages:

```text
config
  parses and validates topology

lab
  orchestrates setup, rollback, and removal

namespace
  manages namespaces, interfaces, routes, and forwarding

helper
  runs namespace-sensitive operations
```

The orchestration layer no longer depends on fixed node names, subnets, interface names, or a single router.

## Integration testing

Integration tests run against real Linux networking state.

Fixtures under:

```text
integration/files/
```

describe increasingly complex topologies.

Tests under:

```text
integration/setup/
```

verify:

- namespaces;
- links;
- addresses;
- routes;
- forwarding;
- representative connectivity paths;
- rollback behavior.

The suite is best run inside a privileged disposable Docker container.

## Key lessons

- A topology description should represent desired state, not encode orchestration logic.
- Connected routes should be left to the kernel.
- Static routes should be explicit rather than inferred.
- Multi-router connectivity depends on both forward and return paths.
- Dual-homing does not imply forwarding between interfaces.
- Rollback must distinguish resources created by Netlab from resources that existed beforehand.
- Integration tests should verify Linux kernel state, not only internal Go state.

## Deferred follow-ups

- validate route gateways against interface subnets;
- reject reuse of one interface endpoint across multiple links;
- add repeated `up` / `down` lifecycle tests;
- add `netlab exec <node> -- <command>`.