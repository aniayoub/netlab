# Stage 3 — Linux Bridge and MAC Learning

## Goal

Replace the direct host-to-host veth topology with a Layer-2 network built around a Linux bridge, then observe how the bridge learns and forwards Ethernet traffic.

The implemented topology is:

```text
host-a namespace                         root namespace                         host-b namespace

10.0.0.1/24                                                                       10.0.0.2/24
    eth0                                                                              eth0
      |                                                                                |
      | veth                                                                           | veth
      |                                                                                |
   port-a ---------------------------------- br0 ----------------------------------- port-b
```

The key question for this stage is:

> Once a host has constructed an Ethernet frame, how does the Linux bridge decide which port should receive it?

---

## Implementation

`netlab up` creates:

```text
network namespaces:
  host-a
  host-b

bridge:
  br0

veth pairs:
  host-a/eth0 ↔ port-a
  host-b/eth0 ↔ port-b
```

The root-side endpoints `port-a` and `port-b` are attached to `br0` and brought up. The host-side endpoints are moved into their respective namespaces, configured as `eth0`, assigned IPv4 addresses, and brought up.

The bridge itself does not need an IPv4 address for this topology. Its role is Layer-2 forwarding between its ports.

The host addresses remain:

```text
host-a/eth0: 10.0.0.1/24
host-b/eth0: 10.0.0.2/24
```

Linux creates the connected `10.0.0.0/24` route inside each namespace from those interface addresses.

---

## Host Neighbor State vs Bridge Forwarding State

Stage 2 introduced the host neighbor table:

```text
ip neigh

IP address → MAC address
```

Stage 3 introduces the bridge forwarding database:

```text
bridge fdb show

MAC address → bridge port
```

These tables solve different problems.

For example, `host-a` may know:

```text
10.0.0.2 → MAC-B
```

while the bridge independently knows:

```text
MAC-B → port-b
```

The complete path is therefore:

```text
host-a
10.0.0.2
   ↓ neighbor lookup / ARP
MAC-B
   ↓ Ethernet frame
br0
MAC-B
   ↓ FDB lookup
port-b
   ↓
host-b
```

The bridge does not perform ARP resolution for the host. ARP is host-side neighbor resolution. The bridge only forwards the resulting Ethernet frames.

---

## MAC Learning

A bridge learns from the **source MAC address** of frames arriving on its ports.

If this frame arrives on `port-a`:

```text
src MAC = MAC-A
dst MAC = MAC-B
```

then the bridge can immediately learn:

```text
MAC-A → port-a
```

It does not need to query the other ports or consult a host neighbor table.

When traffic later arrives from `host-b` through `port-b`, the bridge can learn:

```text
MAC-B → port-b
```

The learned state can be inspected with:

```bash
bridge fdb show br br0
```

The host MAC addresses can be correlated with:

```bash
ip -n host-a link show eth0
ip -n host-b link show eth0
```

---

## Known Unicast Forwarding

If the FDB contains:

```text
MAC-A → port-a
MAC-B → port-b
```

and a frame arrives on `port-a` with destination `MAC-B`, the bridge forwards the frame toward `port-b`.

Conceptually:

```text
frame arrives on port-a
        ↓
destination = MAC-B
        ↓
FDB lookup
        ↓
MAC-B → port-b
        ↓
forward out port-b
```

This is known-unicast forwarding.

---

## Unknown-Unicast Flooding

If the bridge receives a unicast frame whose destination MAC is not present in the FDB, it cannot select one destination port.

For example:

```text
src MAC = MAC-A
dst MAC = MAC-B

FDB:
MAC-A → port-a
MAC-B → unknown
```

The frame is still a unicast frame because its Ethernet destination is a specific MAC address. The bridge handles the unknown destination by flooding copies out the other eligible bridge ports.

This is **unknown-unicast flooding**, which is different from an Ethernet broadcast.

With only two bridge ports, known-unicast forwarding and flooding can look similar from packet captures because there is only one other port. A third host would make the distinction easier to observe.

---

## Broadcast Traffic

ARP requests use the Ethernet broadcast destination:

```text
ff:ff:ff:ff:ff:ff
```

If an ARP request enters `br0` through `port-a`, the bridge forwards it out the other eligible ports in the broadcast domain.

For the current two-host topology:

```text
host-a
  ↓ ARP broadcast
port-a
  ↓
br0
  ↓
port-b
  ↓
host-b
```

The ARP reply then gives `host-a` the destination MAC address it needs for normal unicast IP traffic.

---

## Useful Inspection Commands

Inspect the bridge and its ports:

```bash
ip link show br0
ip link show master br0
bridge link
```

Inspect the forwarding database:

```bash
bridge fdb show br br0
```

Inspect host MAC addresses:

```bash
ip -n host-a link show eth0
ip -n host-b link show eth0
```

Inspect host neighbor state:

```bash
ip -n host-a neigh
ip -n host-b neigh
```

Observe Ethernet, ARP, and ICMP traffic:

```bash
tcpdump -n -e -i port-a 'arp or icmp'
tcpdump -n -e -i port-b 'arp or icmp'
```

Generate traffic:

```bash
ip netns exec host-a ping 10.0.0.2
ip netns exec host-b ping 10.0.0.1
```

---

## Resource Ownership and Rollback

Stage 3 also introduced a more important resource-lifecycle problem than Stage 1: setup now creates several dependent kernel resources and can fail after only part of the topology exists.

The current design treats `lab.Setup()` as the transaction boundary.

Conceptually:

```text
create namespaces
      ↓
create bridge
      ↓
create/configure host-a veth
      ↓
create/configure host-b veth
      ↓
setup succeeds
```

If setup fails, `lab` rolls back the resources that were successfully created by that setup attempt.

Once a veth pair has been created, `VethPair` is returned to the caller even if a later configuration step fails. This keeps the resource visible to the central rollback logic.

The root-side bridge port is used as the authoritative cleanup handle for a veth pair:

```text
port-a ↔ host-a/eth0
port-b ↔ host-b/eth0
```

Deleting one endpoint of a veth pair deletes the pair. Using the root-side port avoids depending on access to an endpoint that has already been moved into a namespace or whose namespace has already been removed.

Cleanup tolerates a veth that has already disappeared as a result of namespace removal.

---

## Integration-Test Coverage

The Stage 3 integration suite verifies kernel state rather than only checking CLI success.

The covered behavior includes:

```text
namespaces exist
loopback interfaces are UP
host eth0 interfaces exist and are UP
host IPv4 addresses are correct
connected routes exist
br0 exists and is UP
port-a and port-b exist and are UP
port-a and port-b have br0 as their bridge master
host-a can reach host-b
host-b can reach host-a
netlab down removes the topology
setup conflicts trigger rollback of resources created by netlab
```

Connectivity is a data-plane property and may become usable shortly after configuration calls return, so tests should distinguish successful configuration from eventual packet-forwarding readiness rather than assuming the first packet emitted immediately after setup must always succeed.

---

## Current Limitation

The namespace-side veth endpoint is currently created with the name `eth0` while it is still in the root namespace and is then moved into the target namespace.

This means setup currently assumes that root `eth0` is available during creation.

A production-oriented implementation would normally use a collision-resistant temporary root-side name and rename the interface after it has been moved into the namespace. For the current learning stage, this is an accepted limitation.

---

## Stage 3 Mental Model

The complete model for same-subnet communication through the bridge is:

```text
host-a wants to send to 10.0.0.2
        ↓
route lookup
        ↓
neighbor lookup
        ↓
ARP if MAC-B is unknown
        ↓
host-a constructs Ethernet frame
src = MAC-A
dst = MAC-B
        ↓
frame enters br0 through port-a
        ↓
bridge learns MAC-A → port-a
        ↓
bridge looks up MAC-B in FDB
        ↓
known                 unknown
  ↓                       ↓
port-b             flood other ports
        ↓
host-b
```

The key separation is:

```text
host networking:
IP → MAC

bridge forwarding:
MAC → port
```

That distinction is the central learning outcome of Stage 3.

---

## Stage 3 Acceptance

Stage 3 is complete when the following can be implemented, observed, and explained:

- two network namespaces communicate through a Linux bridge rather than a direct veth pair;
- the root-side veth endpoints are bridge ports of `br0`;
- the bridge learns source MAC addresses and associates them with ingress ports;
- known destination MACs are forwarded using the FDB;
- unknown unicast destinations are flooded rather than resolved by ARP on behalf of the bridge;
- ARP broadcasts traverse the Layer-2 broadcast domain;
- `ip neigh` and the bridge FDB can be explained as separate pieces of kernel state;
- partial setup failures roll back resources owned by the current `netlab up` attempt;
- integration tests verify the intended kernel topology and connectivity.
