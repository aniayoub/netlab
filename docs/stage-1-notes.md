# Stage 1: Two virtual Hosts
## Objective
Create two Linux network namespaces and connect them by a veth pair:
host-a (10.0.0.1/24) <----veth----> host-b(10.0.0.2/24)
The objective is to understance Linux namespaces isolation and how to establish comminucation between them

## Observations

* A veth pair creates two kernel network interfaces. `netlink.Veth` is a Go representation/configuration, not a single kernel-side veth object.
* A newly created link belongs to the network namespace in which it is created.
* `netns.NewNamed` creates the namespace and switches the calling OS thread into it.
* A network interface belongs to one network namespace at a time.
* Moving an interface to another namespace brings the interface down; it must be brought up again in the destination namespace.
* In this experiment, an IP address configured before moving the interface was not retained after the move. I therefore configure addresses after moving interfaces into their final namespaces.
* `netlink.LinkByName` resolves links in the current network namespace.
* Deleting one endpoint of a veth pair deletes its peer.
* Retaining a `netlink.Link` value does not bypass namespace isolation. After the interface moves, operations must be performed from the namespace that owns it.
