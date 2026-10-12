package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"gopkg.in/yaml.v3"
)

type ConfigDto struct {
	Nodes []NodeDto
	Links []NodeLinkDto
}

type Config struct {
	Nodes map[string]*NodeConfig
	Links []*NodeLink
}

func Parse(yamlPath string) (*Config, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}

	var dto ConfigDto
	err = yaml.Unmarshal(data, &dto)
	if err != nil {
		return nil, err
	}

	cfg, err := dto.buildConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (dto *ConfigDto) buildConfig() (*Config, error) {
	nodes := make(map[string]*NodeConfig)
	links := []*NodeLink{}

	seenNodes := make(map[string]struct{})
	nodeInterfaces := make(map[string]map[string]struct{})

	// 1. Validate Unique Node Names & Unique Interfaces per Node
	for _, nodeDto := range dto.Nodes {
		if nodeDto.Type != Router && nodeDto.Type != Host {
			return nil, fmt.Errorf("invalid node type %q for node %q", nodeDto.Type, nodeDto.Name)
		}

		if _, exists := seenNodes[nodeDto.Name]; exists {
			return nil, fmt.Errorf("duplicate node name found: %s", nodeDto.Name)
		}
		seenNodes[nodeDto.Name] = struct{}{}

		node, err := nodeDto.ToNodeConfig()
		if err != nil {
			return nil, err
		}

		nodes[nodeDto.Name] = node
		nodeInterfaces[nodeDto.Name] = make(map[string]struct{})
		for ifaceName := range node.Interfaces {
			nodeInterfaces[nodeDto.Name][ifaceName] = struct{}{}
		}
	}

	// 2. Validate Links using the "node:interface" endpoints format
	for _, link := range dto.Links {
		if len(link.Endpoints) != 2 {
			return nil, fmt.Errorf("link must have exactly 2 endpoints, got %d", len(link.Endpoints))
		}

		validateEndpoint := func(endpoint string) error {
			parts := strings.Split(endpoint, ":")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("invalid endpoint format %q (expected node:interface)", endpoint)
			}

			nodeName := parts[0]
			ifaceName := parts[1]

			// Check if node exists
			ifaces, nodeExists := nodeInterfaces[nodeName]
			if !nodeExists {
				return fmt.Errorf("link references non-existent node: %s", nodeName)
			}

			// Check if interface exists on that node
			if _, ifaceExists := ifaces[ifaceName]; !ifaceExists {
				return fmt.Errorf("link references non-existent interface %q on node %q", ifaceName, nodeName)
			}

			return nil
		}

		for _, endpoint := range link.Endpoints {
			if err := validateEndpoint(endpoint); err != nil {
				return nil, err
			}

		}
		links = append(links, link.ParseEndpoints())
	}

	return &Config{
		Nodes: nodes,
		Links: links,
	}, nil
}

func (dto *NodeDto) ToNodeConfig() (*NodeConfig, error) {
	interfaces := make(map[string]*InterfaceConfig)
	routes := []*RouteConfig{}

	for _, iface := range dto.Interfaces {
		// Validate uniqueness of interface names
		if _, exists := interfaces[iface.Name]; exists {
			return nil, fmt.Errorf("duplicate interface name %q on node %q", iface.Name, dto.Name)
		}
		// Validate that the IP is a valid CIDR before parsing
		ipNet, err := netlink.ParseIPNet(iface.IP)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q on interface %q of node %q: %v", iface.IP, iface.Name, dto.Name, err)
		}

		interfaces[iface.Name] = &InterfaceConfig{
			Name:  iface.Name,
			IpNet: ipNet,
		}
	}

	for _, route := range dto.Routes {

		interfaceConfig := interfaces[route.Interface]

		if interfaceConfig == nil {
			return nil, fmt.Errorf("no interface found on node %q for route with gateway %q", dto.Name, route.Gateway)
		}

		// validate that the destination is a valid CIDR
		_, _, err := net.ParseCIDR(route.Destination)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q on route of node %q: %v", route.Destination, dto.Name, err)
		}

		// Validate gateway is a valid IP address
		if net.ParseIP(route.Gateway) == nil {
			return nil, fmt.Errorf("invalid IP %q on route of node %q", route.Gateway, dto.Name)
		}
		routes = append(routes, &route)
	}

	return &NodeConfig{
		Name:       dto.Name,
		Type:       dto.Type,
		Interfaces: interfaces,
		Routes:     routes,
	}, nil
}
