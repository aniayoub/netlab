# Netlab — Programmable Linux Networking Lab

## Overview

Build a small **programmable Linux network lab** that creates reproducible virtual network topologies on a single Linux host.

Conceptually:

```text
netlab
  ├─ creates virtual hosts and routers
  ├─ connects them with virtual links
  ├─ configures addressing and routes
  ├─ runs connectivity experiments
  ├─ injects network failures
  └─ exposes enough observability to explain packet flow
```

The implementation can use Linux network namespaces, veth pairs, bridges, routing tables, `tc`, and standard diagnostic tools.

The important distinction from a container runtime is:

```text
container runtime → isolate one process environment

netlab → construct and reason about an entire network
```

## Objective

The primary goal is not to build a production SDN controller or router.

The goal is to become capable of answering questions like:

- What exact path does this packet take?
- Why does ARP happen here but not there?
- Why is this route selected?
- Why can A reach B but B cannot reach A?
- What happens when an MTU is wrong?
- How does latency or loss affect TCP?
- How do bridges differ from routers?
- How do I prove where connectivity is failing using packet captures and kernel state?

For portfolio purposes, the project should demonstrate that an experienced software engineer can reason below the application layer, build reproducible network environments, diagnose failures from first principles, and understand how Linux implements networking primitives.

---

# Stages

## Stage 0 — Network inspection

Before building anything, inspect the host:

```bash
ip link
ip addr
ip route
ip neigh
ss
```

Also inspect relevant `/proc/net/*` state.

Learning goal:

```text
interface
address
route
neighbor
socket
```

Understand what each represents before the tool starts creating them.

---

## Stage 1 — Two virtual hosts

Create:

```text
host-a ←── veth ──→ host-b
```

Each side lives in its own network namespace.

Configure:

```text
host-a: 10.0.0.1/24
host-b: 10.0.0.2/24
```

Bring interfaces and loopback up.

Acceptance test:

```text
host-a → ping → host-b
```

Concepts:

- network namespaces
- veth pairs
- interface state
- IP addressing
- connected routes

---

## Stage 2 — Observe ARP and packet flow

Do not add new infrastructure yet.

Use:

```bash
ip neigh
tcpdump
ping
```

Observe:

```text
ARP request
→ ARP reply
→ ICMP echo request
→ ICMP echo reply
```

Deliberately clear neighbor entries and watch them rebuild.

Concepts:

- Ethernet
- MAC addresses
- ARP
- L2 vs L3
- packet capture

The emphasis is on observation, not merely configuration.

---

## Stage 3 — Build a virtual switch

Topology:

```text
host-a ─┐
        ├── bridge0
host-b ─┘
```

Optionally add `host-c`.

Create a Linux bridge and attach veth endpoints.

Concepts:

- switches
- bridges
- MAC learning
- broadcast domains
- forwarding databases

Experiments:

```bash
bridge fdb show
tcpdump
```

Observe ARP broadcasts, learned MAC addresses, and behavior when ports are removed or added.

---

## Stage 4 — Introduce routing

Topology:

```text
host-a                  host-b
10.0.1.2                10.0.2.2
   │                       │
   │ 10.0.1.1     10.0.2.1 │
   └────── router ──────────┘
```

Configure:

```text
host-a default via 10.0.1.1
host-b default via 10.0.2.1
router forwarding enabled
```

Concepts:

- routing tables
- next hop
- gateways
- longest-prefix match
- IP forwarding
- TTL

This is one of the most important stages in the project.

---

## Stage 5 — Turn it into an actual tool

Up to this point, scripts and manual commands are acceptable.

Now introduce a simple topology description, for example:

```yaml
nodes:
  host-a:
    type: host
  router:
    type: router
  host-b:
    type: host

links:
  - [host-a, router]
  - [router, host-b]
```

Possible CLI:

```bash
netlab up topology.yaml
netlab exec host-a ping 10.0.2.2
netlab inspect
netlab down
```

Do not over-engineer the schema.

Learning goals:

- translating topology into kernel state
- resource ownership
- cleanup
- idempotency
- orchestration

---

## Stage 6 — Network observability

Add commands or documented workflows for inspecting:

```text
interfaces
addresses
routes
neighbors
bridge FDB
sockets
packet captures
```

For example:

```bash
netlab inspect host-a
```

could show a concise summary of:

```text
links
addresses
routes
neighbors
```

Do not hide the underlying Linux tools too much; learning them is part of the project.

---

## Stage 7 — Failure injection

Introduce:

```text
latency
packet loss
link failure
```

Use `tc netem` where appropriate.

Experiments:

```text
0 ms latency
→ 100 ms latency
→ 2% packet loss
```

Observe:

```text
ping
TCP throughput
retransmissions
connection behavior
```

This is where the project starts demonstrating strong systems-engineering signal.

---

## Stage 8 — MTU failures

Deliberately create an MTU mismatch.

Example:

```text
host A MTU 1500
middle link MTU 1200
```

Investigate:

```text
fragmentation
DF bit
ICMP fragmentation-needed
PMTU discovery
```

Observe behavior at different packet sizes and diagnose the failure from packet captures and interface state.

---

# MVP Boundary

The MVP ends when the project has:

- network namespaces
- veth
- IP addressing
- ARP observation
- Linux bridge
- multi-subnet routing
- declarative/reproducible topology
- packet inspection
- latency/loss injection
- MTU experiment
- reliable cleanup
- integration tests

A strong MVP demo topology:

```text
host-a
   │
switch-a
   │
router
   │
switch-b
   │
host-b
```

Demonstrate:

```text
connectivity
→ inspect route
→ inspect ARP
→ capture packets
→ inject 100 ms latency
→ inject packet loss
→ break MTU
→ diagnose each problem
```

## Explicitly out of MVP

- NAT
- VXLAN
- BGP
- eBPF/XDP
- distributed control plane
- Kubernetes/CNI-style networking

---

# Stretch Goals

## Stretch A — NAT

Add a node that reaches the host or external network through:

```text
SNAT / masquerading
connection tracking
```

Learn:

- nftables
- conntrack
- source NAT
- destination NAT

---

## Stretch B — TCP experiments

Investigate:

```text
handshake
retransmission
TIME_WAIT
listen backlog
slow receiver
congestion behavior
```

A small traffic generator may be useful, but avoid turning the project into a TCP implementation exercise.

---

## Stretch C — VXLAN overlay

Build:

```text
underlay IP network
        ↓
VXLAN tunnels
        ↓
logical L2 network
```

This introduces the foundation of modern data-center overlays.

---

## Stretch D — ECMP

Create multiple equal-cost routes and investigate:

```text
flow hashing
path selection
failure behavior
```

---

## Stretch E — eBPF/XDP

Only after the core networking model is well understood.

Use eBPF/XDP for a concrete purpose such as:

```text
packet counters
flow visibility
simple filtering
```

Do not add it merely because it is fashionable.

---

## Stretch F — Visualization

Generate a graph from the topology description showing:

```text
nodes
links
addresses
routes
```

This adds significant portfolio presentation value without changing the networking fundamentals.

---

# What to Avoid

Do not turn the project into:

```text
mini Kubernetes networking
```

or:

```text
a complete router implementation
```

or:

```text
a TCP stack
```

The learning loop should stay:

```text
construct
→ predict
→ inspect
→ break
→ capture packets
→ explain
→ automate
```

The project succeeds when each stage leaves behind a working, observable topology and a networking concept that can be explained from first principles.
