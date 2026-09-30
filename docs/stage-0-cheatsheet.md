# Netlab — Stage 0 Cheatsheet

## Core mental model

process
  ↓
socket
  ↓
destination IP
  ↓
routing decision
  ↓
next-hop IP
  ↓
neighbor lookup
  ↓
next-hop MAC
  ↓
output interface

---

# 1. Interfaces

## Commands

ip link
ip -br link
ip -details link
ip -s link
ip link show <interface>

## Important fields

### ifindex

Example:

2: eth0

`2` is the kernel interface index.

Also visible at:

/sys/class/net/<interface>/ifindex

### UP

Administrative state.

UP means:

"The interface has been enabled."

It does NOT guarantee that the underlying link works.

### LOWER_UP

The lower layer is operational.

For Ethernet, this often corresponds to having carrier/link.

Possible situation:

UP + no LOWER_UP

means approximately:

"enabled, but there is no usable underlying link"

### state UP / DOWN / UNKNOWN

Operational state.

Important:

administrative UP != operational UP

Loopback commonly reports:

state UNKNOWN

because it does not have a normal physical carrier.

### MTU

Maximum Transmission Unit.

Rough mental model:

largest IP packet the interface can transmit without requiring
special handling because of size.

Typical Ethernet:

MTU 1500

Loopback commonly has a much larger MTU.

MTU does NOT mean total Ethernet frame size.

### link/ether

Example:

link/ether aa:bb:cc:dd:ee:ff

The interface's Ethernet MAC address.

---

# 2. IP addresses

## Commands

ip addr
ip -br addr
ip addr show dev <interface>

## Mental model

interface
  ↓
zero or more IP addresses

Interface != IP address.

Example:

192.168.1.42/24

address:
192.168.1.42

prefix length:
/24

network prefix:
192.168.1.0/24

## Prefix

IPv4 = 32 bits.

Example:

/24

24 network bits
8 host bits

## Common scopes

scope host
    only meaningful inside the host

scope link
    meaningful on the local link

scope global
    normal routable address scope

IMPORTANT:

"global" does NOT mean "public Internet IP".

Private addresses can have scope global.

---

# 3. Routing

## Commands

ip route
ip route get <destination>

## Mental model

A route answers:

"For destination X, where should the packet go next?"

## Connected route

Example:

192.168.1.0/24 dev wlan0 proto kernel scope link src 192.168.1.42

Meaning approximately:

Destination:
    192.168.1.0/24

Output interface:
    wlan0

Next hop:
    none; destination is directly reachable

Preferred source:
    192.168.1.42

## Default route

Example:

default via 192.168.1.1 dev wlan0

Meaning:

For destinations without a more specific route:

next hop:
    192.168.1.1

interface:
    wlan0

`default` means:

0.0.0.0/0

## Longest-prefix match

If multiple routes match:

the most specific prefix wins.

Example:

192.168.1.0/24
0.0.0.0/0

Destination:
192.168.1.50

/24 wins over /0.

## Key distinction

final destination != next hop

Example:

Final destination:
8.8.8.8

Next hop:
192.168.1.1

## Best diagnostic command

ip route get <IP>

Example:

ip route get 8.8.8.8

Answers:

- output interface
- next hop
- source address
- selected route

---

# 4. Neighbors

## Command

ip neigh

## Mental model

Routing gives:

destination IP
    ↓
next-hop IP

Neighbor state gives:

next-hop IP
    ↓
link-layer address

For IPv4 + Ethernet this commonly means:

IP → MAC

More precisely:

(IP, interface) → link-layer information

## Example

192.168.1.1 dev wlan0 lladdr aa:bb:cc:dd:ee:ff REACHABLE

Meaning:

neighbor IP:
    192.168.1.1

interface:
    wlan0

MAC:
    aa:bb:cc:dd:ee:ff

state:
    REACHABLE

## Common states

REACHABLE
    recently confirmed reachable

STALE
    mapping is cached but not recently confirmed

FAILED
    neighbor resolution failed

STALE != broken

## Remote destination example

Want to reach:

8.8.8.8

Route says:

via 192.168.1.1

Neighbor lookup is for:

192.168.1.1

NOT:

8.8.8.8

The gateway is your local neighbor.

---

# 5. Sockets

## TCP listeners

ss -lnt

Flags:

-l = listening
-n = numeric addresses/ports
-t = TCP

## Established TCP connections

ss -tn

## UDP

ss -lun

-u = UDP

## Process information

ss -lntp
ss -tnp

-p = process information, when permissions allow

## Listener bindings

127.0.0.1:8080

means:

listen specifically on loopback

0.0.0.0:8080

means:

IPv4 wildcard binding

approximately:

listen on any appropriate local IPv4 address

## TCP connection

Example:

192.168.1.42:48123 → 203.0.113.10:443

Local endpoint:

192.168.1.42:48123

Remote endpoint:

203.0.113.10:443

48123 is likely an ephemeral client port.

443 is the server port.

## TCP connection identity

Think roughly:

protocol
+ source IP
+ source port
+ destination IP
+ destination port

---

# 6. /sys

## Interfaces

ls /sys/class/net

Useful files:

/sys/class/net/<interface>/ifindex
/sys/class/net/<interface>/mtu
/sys/class/net/<interface>/operstate
/sys/class/net/<interface>/address

Useful relationship:

ip link
    human-friendly interface view

/sys/class/net/*
    individual kernel-exposed attributes

---

# 7. /proc networking state

## Interface statistics

cat /proc/net/dev

Compare with:

ip -s link

Shows things such as:

RX bytes
RX packets
RX errors
RX drops

TX bytes
TX packets
TX errors
TX drops

## IPv4 routing

cat /proc/net/route

Compare with:

ip route

Prefer `ip route` for human inspection.

## IPv4 ARP

cat /proc/net/arp

Compare with:

ip neigh

Prefer `ip neigh` because it represents the broader neighbor abstraction.

## TCP sockets

cat /proc/net/tcp
cat /proc/net/tcp6

Compare with:

ss -tn

## UDP sockets

cat /proc/net/udp
cat /proc/net/udp6

Compare with:

ss -un

---

# 8. Diagnostic reasoning

Given a remote TCP connection:

1. Socket

ss -tn

Ask:

What is the remote IP and port?

2. Route

ip route get <REMOTE_IP>

Ask:

Which interface?
Which source IP?
Directly connected or via gateway?
What is the next hop?

3. Neighbor

ip neigh

Ask:

Do I know the link-layer address of the next hop?

4. Interface

ip link show <interface>

Ask:

Is it administratively UP?
Is it operationally UP?
What is its MTU?

5. Address

ip addr show dev <interface>

Ask:

Does the interface have the expected source address?

---

# 9. Direct vs routed delivery

## Same local subnet

destination IP
    ↓
directly connected route
    ↓
destination itself is the next hop
    ↓
neighbor lookup for destination
    ↓
destination MAC

## Remote network

destination IP
    ↓
default/more-specific route
    ↓
gateway is next hop
    ↓
neighbor lookup for gateway
    ↓
gateway MAC

IMPORTANT:

Remote IP stays the IP destination.

Gateway MAC is only the local Ethernet destination.

---

# 10. Stage 0 object summary

## Interface

Question:

"What networking endpoint/link does Linux have?"

Tool:

ip link

---

## Address

Question:

"What IP identities are assigned locally?"

Tool:

ip addr

---

## Route

Question:

"For destination X, where should the packet go next?"

Tool:

ip route
ip route get <IP>

---

## Neighbor

Question:

"How do I reach this directly adjacent IP on this link?"

Tool:

ip neigh

---

## Socket

Question:

"What transport endpoints are applications using?"

Tool:

ss

---

# Debugging mnemonic

socket
→ destination
→ route
→ next hop
→ neighbor
→ interface

When something fails, avoid starting with:

"The network is broken."

Ask instead:

1. Is the socket/state what I expect?
2. Is the destination correct?
3. Which route did Linux choose?
4. What is the next hop?
5. Does neighbor resolution exist?
6. Is the selected interface usable?