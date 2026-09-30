# Stage 2 — ARP and Packet Flow

## Goal

Understand what Linux does when one host sends traffic to another host on the same IPv4 subnet.

The lab topology is:

```text
host-a                              host-b
10.0.0.1/24                        10.0.0.2/24
veth0  <========================>  veth1
```

The main question for this stage is:

> How does `host-a` send an IP packet to `10.0.0.2` when Ethernet requires a destination MAC address?

---

## Mental Model

When `host-a` sends traffic to `10.0.0.2`, Linux roughly performs these steps:

```text
application generates traffic
        ↓
route lookup
        ↓
determine next hop and interface
        ↓
neighbor lookup
        ↓
ARP resolution if MAC is unknown
        ↓
construct Ethernet frame
        ↓
send IPv4 packet
```

For this topology, `10.0.0.2` is directly connected to `host-a`.

Linux therefore needs the MAC address associated with `10.0.0.2` before it can transmit the Ethernet frame.

---

## Experiment 1 — Cold Neighbor Cache

First, remove any existing neighbor information:

```bash
ip netns exec host-a ip neigh flush all
```

Confirm that there is no entry for `10.0.0.2`:

```bash
ip -n host-a neigh
```

Inspect the MAC addresses of both interfaces:

```bash
ip -n host-a link show veth0
ip -n host-b link show veth1
```

Start a packet capture on `host-a`:

```bash
ip netns exec host-a tcpdump -n -e -i veth0 'arp or icmp'
```

From another terminal, generate a single ICMP request:

```bash
ip netns exec host-a ping -c 1 10.0.0.2
```

The capture shows approximately:

```text
ARP Request: Who has 10.0.0.2? Tell 10.0.0.1
ARP Reply:   10.0.0.2 is at <host-b MAC>

ICMP Echo Request: 10.0.0.1 → 10.0.0.2
ICMP Echo Reply:   10.0.0.2 → 10.0.0.1
```

After the ping:

```bash
ip -n host-a neigh
```

now contains an entry mapping:

```text
10.0.0.2 → <host-b MAC>
```

The MAC address in the neighbor table matches the MAC address assigned to `host-b`'s `veth1` interface.

---

## What Happened

`ping` wants to send an IPv4 packet to:

```text
10.0.0.2
```

Before Linux can transmit that packet over Ethernet, it needs to create an Ethernet frame containing a destination MAC address.

Initially, `host-a` knows:

```text
destination IP = 10.0.0.2
destination MAC = unknown
```

Linux therefore sends an ARP request.

The request is sent to the Ethernet broadcast address:

```text
ff:ff:ff:ff:ff:ff
```

Conceptually:

```text
Who owns 10.0.0.2?
Tell 10.0.0.1.
```

`host-b` owns that IP address and sends an ARP reply containing its MAC address.

Linux then caches the mapping in the neighbor table:

```text
10.0.0.2 → <host-b MAC>
```

It can now construct the Ethernet frame carrying the ICMP packet.

Conceptually:

```text
Ethernet
├── source MAC: host-a MAC
├── destination MAC: host-b MAC
└── IPv4
    ├── source IP: 10.0.0.1
    ├── destination IP: 10.0.0.2
    └── ICMP Echo Request
```

ARP is therefore not part of the ICMP packet. It is separate traffic used to discover the link-layer address required to deliver the IP packet.

---

## Experiment 2 — Warm Neighbor Cache

Without flushing the neighbor table, start another capture:

```bash
ip netns exec host-a tcpdump -n -e -i veth0 'arp or icmp'
```

Send another ping:

```bash
ip netns exec host-a ping -c 1 10.0.0.2
```

This time the traffic can normally proceed without another ARP exchange.

The neighbor table already contains:

```text
10.0.0.2 → <host-b MAC>
```

Linux can immediately construct the Ethernet frame.

The expected packet flow is therefore:

```text
ICMP Echo Request
ICMP Echo Reply
```

rather than:

```text
ARP Request
ARP Reply
ICMP Echo Request
ICMP Echo Reply
```

This demonstrates why the neighbor information is cached.

---

## Experiment 3 — Force Resolution Again

Flush the neighbor table:

```bash
ip netns exec host-a ip neigh flush all
```

Run the capture again:

```bash
ip netns exec host-a tcpdump -n -e -i veth0 'arp or icmp'
```

Send another ping:

```bash
ip netns exec host-a ping -c 1 10.0.0.2
```

The ARP request and reply appear again before ICMP traffic.

This confirms that ARP was triggered because Linux lacked the required neighbor information.

---

## Route Lookup Comes First

ARP resolution is not the first decision Linux makes.

Before resolving a MAC address, Linux determines how the destination IP should be reached.

Inspect the routing table:

```bash
ip -n host-a route
```

The lab contains a connected route similar to:

```text
10.0.0.0/24 dev veth0 scope link src 10.0.0.1
```

Ask Linux specifically how it would reach `10.0.0.2`:

```bash
ip -n host-a route get 10.0.0.2
```

The result identifies `veth0` as the outgoing interface.

Because `10.0.0.2` belongs to the directly connected `10.0.0.0/24` network, Linux treats the destination itself as the next hop.

The resulting sequence is:

```text
destination: 10.0.0.2
        ↓
route lookup
        ↓
directly reachable through veth0
        ↓
neighbor lookup for 10.0.0.2
        ↓
MAC cached?
   │          │
  yes         no
   │          ↓
   │         ARP
   │          ↓
   └────── MAC learned
              ↓
       Ethernet frame
              ↓
          IP / ICMP
```

This distinction becomes important when routing through a gateway.

For an off-link destination, the system does not normally ARP for the final destination IP. It resolves the MAC address of the next-hop router instead.

---

## Evidence Correlation

The same MAC address can be observed through several independent views of the system.

### Interface configuration

```bash
ip -n host-b link show veth1
```

Shows the MAC address assigned to `host-b`.

### ARP traffic

```bash
ip netns exec host-a tcpdump -n -e -i veth0 arp
```

Shows `host-b` advertising that MAC address in its ARP reply.

### Neighbor table

```bash
ip -n host-a neigh
```

Shows Linux caching the same MAC address against `10.0.0.2`.

Therefore:

```text
host-b interface MAC
        =
MAC advertised in ARP reply
        =
MAC stored in host-a neighbor table
```

This is useful operationally because the same networking state can be inspected from both kernel state and observed wire traffic.

---

## Key Takeaways

ARP solves a specific problem on IPv4 Ethernet networks:

> Given the IPv4 address of an on-link next hop, determine the link-layer address required to send it an Ethernet frame.

The important sequence is:

```text
route lookup
→ determine next hop
→ neighbor lookup
→ ARP if necessary
→ Ethernet transmission
```

`ip route` answers:

> Where should this packet go?

`ip neigh` answers:

> What link-layer information does the kernel currently know about that neighbor?

`tcpdump` answers:

> What traffic actually crossed the interface?

Together, these tools provide complementary views of Linux networking behavior.

## Stage 2 Acceptance

Stage 2 is complete when the following behavior can be reproduced and explained:

- With no neighbor entry, the first ping causes ARP resolution.
- The ARP reply provides the MAC address belonging to `host-b`.
- Linux stores the resulting IP-to-MAC mapping in its neighbor table.
- ICMP traffic is then carried in Ethernet frames addressed to that MAC.
- A subsequent ping can reuse the cached neighbor information without performing ARP again.
- Removing the neighbor entry causes ARP resolution to happen again.
- The route lookup explains why Linux resolves `10.0.0.2` directly instead of resolving a router.