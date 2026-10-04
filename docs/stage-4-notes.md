# Stage 4 — IPv4 Routing Between Subnets

## Goal

Stage 4 moves the lab from Layer-2 switching to Layer-3 routing.

The topology contains two hosts on different IPv4 subnets with a Linux network namespace acting as the router between them.

```text
10.0.1.0/24                              10.0.2.0/24

host-a                 router                  host-b
10.0.1.2/24       10.0.1.1/24  10.0.2.1/24    10.0.2.2/24
   eth0  <-------->  port-a      port-b  <--------> eth0

 default via                                  default via
 10.0.1.1                                     10.0.2.1
```

There is no bridge in this stage. Each host is directly connected to one router interface through a veth pair.

The main question for this stage is:

> How does Linux deliver an IP packet when the destination is not on the sender's directly connected network?

---

## Implemented Topology

`netlab up` creates three named network namespaces:

```text
host-a
router
host-b
```

Two veth pairs provide the links:

```text
eth-a  <-> port-a
eth-b  <-> port-b
```

After creation, the endpoints are moved into namespaces:

```text
host-a:
    eth-a -> renamed eth0

router:
    port-a
    port-b

host-b:
    eth-b -> renamed eth0
```

The interfaces are configured as:

```text
host-a/eth0      10.0.1.2/24
router/port-a    10.0.1.1/24

router/port-b    10.0.2.1/24
host-b/eth0      10.0.2.2/24
```

Assigning those addresses causes Linux to create the directly connected routes automatically.

The hosts additionally receive default routes:

```text
host-a:
default via 10.0.1.1 dev eth0

host-b:
default via 10.0.2.1 dev eth0
```

IPv4 forwarding is enabled inside the router namespace through:

```text
/proc/sys/net/ipv4/ip_forward
```

---

## Routing Decision on host-a

When `host-a` wants to send traffic to `10.0.2.2`, it first performs a route lookup.

Its routing table is conceptually:

```text
10.0.1.0/24 dev eth0
default via 10.0.1.1 dev eth0
```

`10.0.2.2` does not match the directly connected `10.0.1.0/24` route, so Linux selects the default route.

The next hop is therefore:

```text
10.0.1.1
```

not the final destination `10.0.2.2`.

This can be inspected with:

```bash
ip -n host-a route
ip -n host-a route get 10.0.2.2
```

The important distinction is:

```text
final IP destination: 10.0.2.2
next hop:              10.0.1.1
```

---

## ARP Resolves the Next Hop

Because the next hop is the router interface on the local network, `host-a` needs the router's MAC address before it can transmit the Ethernet frame.

With an empty neighbor cache, `host-a` therefore ARPs for:

```text
10.0.1.1
```

It does not ARP for:

```text
10.0.2.2
```

because `10.0.2.2` is not directly reachable on `host-a`'s Ethernet link.

Conceptually:

```text
host-a route lookup
        ↓
next hop = 10.0.1.1
        ↓
neighbor lookup
        ↓
10.0.1.1 -> router port-a MAC
```

Useful commands:

```bash
ip -n host-a neigh
ip netns exec host-a tcpdump -n -e -i eth0 'arp or icmp'
```

This connects the Stage 2 neighbor-resolution model to routing: ARP resolves the Layer-2 address of the selected next hop, not necessarily the final IP destination.

---

## Router Forwarding

The router receives a frame addressed to its `port-a` MAC containing an IPv4 packet whose destination remains:

```text
10.0.2.2
```

Because IPv4 forwarding is enabled, Linux is allowed to forward packets that are neither sourced from nor destined for the router itself.

The router performs its own route lookup.

Its connected routes are conceptually:

```text
10.0.1.0/24 dev port-a
10.0.2.0/24 dev port-b
```

`10.0.2.2` matches the second route, so Linux forwards the packet through `port-b`.

If the router does not already know the Layer-2 address for `10.0.2.2`, it performs ARP on the `10.0.2.0/24` link and learns:

```text
10.0.2.2 -> host-b MAC
```

The packet can then be transmitted to `host-b`.

Useful commands:

```bash
ip -n router route
ip -n router route get 10.0.2.2
ip -n router neigh
```

---

## L2 Changes at the Router; L3 Destination Does Not

Routing does not simply forward the original Ethernet frame unchanged.

For a ping from `host-a` to `host-b`, the first link carries a frame resembling:

```text
Ethernet
    src MAC = host-a MAC
    dst MAC = router port-a MAC

IPv4
    src IP = 10.0.1.2
    dst IP = 10.0.2.2
```

After the router forwards the packet onto the second link, the Ethernet header is different:

```text
Ethernet
    src MAC = router port-b MAC
    dst MAC = host-b MAC

IPv4
    src IP = 10.0.1.2
    dst IP = 10.0.2.2
```

Therefore:

```text
Ethernet source/destination
    change at each routed link

IP source/destination
    continue to identify the original endpoints
```

This can be observed by capturing the same flow on both router interfaces:

```bash
ip netns exec router tcpdump -n -e -i port-a icmp
ip netns exec router tcpdump -n -e -i port-b icmp
```

---

## TTL

A router also modifies the IPv4 packet itself in one important way: it decrements the packet's TTL before forwarding it.

Conceptually:

```text
host-a -> router port-a
TTL = N

router port-b -> host-b
TTL = N - 1
```

This prevents packets from circulating indefinitely if a routing loop exists.

Verbose packet capture can expose the value:

```bash
ip netns exec router tcpdump -n -vv -i port-a icmp
ip netns exec router tcpdump -n -vv -i port-b icmp
```

---

## Why IPv4 Forwarding Matters

Having two interfaces and routes is not enough to make Linux forward traffic between them.

The router namespace explicitly enables:

```text
net.ipv4.ip_forward = 1
```

The implementation writes the namespaced procfs value:

```text
/proc/sys/net/ipv4/ip_forward
```

It can be inspected with:

```bash
ip netns exec router cat /proc/sys/net/ipv4/ip_forward
```

Without forwarding enabled, the router can communicate using its own interface addresses, but it does not act as a transit router for packets between the hosts.

---

## Route Selection and Longest-Prefix Match

A routing table can contain multiple routes matching the same destination.

Linux chooses the most specific matching prefix.

For example, on `host-a`:

```text
10.0.1.0/24 dev eth0
0.0.0.0/0 via 10.0.1.1
```

Traffic to `10.0.1.1` matches both entries, but `/24` is more specific than `/0`, so the directly connected route wins.

Traffic to `10.0.2.2` does not match `10.0.1.0/24`, so the default route is selected.

`ip route get` is useful for asking Linux for the actual result rather than reasoning only from the table:

```bash
ip -n host-a route get 10.0.1.1
ip -n host-a route get 10.0.2.2
```

---

## Integration Coverage

The Stage 4 integration suite verifies the resulting kernel state rather than only checking application-level success.

It currently covers:

- creation of `host-a`, `router`, and `host-b`;
- placement of host and router interfaces into the expected namespaces;
- removal of the temporary root-side interfaces after setup;
- interface administrative state;
- IPv4 addressing on all four routed interfaces;
- connected routes for both `/24` networks;
- host default routes through the router;
- IPv4 forwarding in the router namespace;
- bidirectional connectivity between `host-a` and `host-b`;
- cleanup through `netlab down`;
- rollback when namespace creation or veth creation encounters resource conflicts;
- preservation of conflicting resources that existed before Netlab setup.

The connectivity test uses multiple ICMP requests because link and neighbor readiness immediately after topology creation can be asynchronous even after the configuration operations themselves have succeeded.

---

## Implementation Structure

Stage 4 simplified the internal architecture.

The current implementation is centered around:

```text
internal/lab
    topology orchestration
    veth creation
    routing setup
    rollback

internal/namespace
    namespace lifecycle
    namespace-local interface configuration
    default routes
    IPv4 forwarding

internal/helper
    execute an operation inside a target network namespace
```

The lab owns the topology and coordinates rollback if setup fails.

Linux remains responsible for the actual networking behavior:

```text
routing lookup
ARP / neighbor resolution
IPv4 forwarding
TTL processing
Ethernet transmission
```

Netlab configures those kernel primitives rather than implementing its own forwarding logic.

---

## Stage 4 Mental Model

The complete forwarding path from `host-a` to `host-b` is:

```text
application generates traffic for 10.0.2.2
        ↓
host-a route lookup
        ↓
default route selects next hop 10.0.1.1
        ↓
host-a resolves router MAC with ARP if necessary
        ↓
Ethernet frame sent to router port-a
        ↓
router receives IPv4 packet
        ↓
router decrements TTL
        ↓
router route lookup selects port-b
        ↓
router resolves host-b MAC if necessary
        ↓
router builds a new Ethernet frame
        ↓
host-b receives packet for 10.0.2.2
```

The key separation is:

```text
routing table
IP destination -> next hop / outgoing interface

neighbor table
next-hop IP -> MAC address

Ethernet
MAC address -> delivery across one local link
```

Stage 4 is complete when this path can be reproduced, inspected, and explained from both routing-table state and packet captures.