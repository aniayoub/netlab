package config

import "net"

type NsType string

const (
	Host   NsType = "host"
	Router NsType = "router"
)

type InterfaceDto struct {
	Name string
	IP   string
}
type RouteConfig struct {
	Interface   string
	Destination string
	Gateway     string
}

type NodeDto struct {
	Name       string
	Type       NsType
	Interfaces []InterfaceDto
	Routes     []RouteConfig
}

type InterfaceConfig struct {
	Name  string
	IpNet *net.IPNet
}

type NodeConfig struct {
	Name       string
	Type       NsType
	Interfaces map[string]*InterfaceConfig
	Routes     []*RouteConfig
}
