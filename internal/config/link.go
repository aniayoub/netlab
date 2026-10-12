package config

import "strings"

type NodeLinkDto struct {
	Endpoints []string
}

type NodeLink struct {
	SrcNode      string
	SrcInterface string
	DstNode      string
	DstInterface string
}

func (link *NodeLinkDto) ParseEndpoints() *NodeLink {
	// We assume the link has exactly two endpoints in the format "node:interface"
	return &NodeLink{
		SrcNode:      strings.Split(link.Endpoints[0], ":")[0],
		SrcInterface: strings.Split(link.Endpoints[0], ":")[1],
		DstNode:      strings.Split(link.Endpoints[1], ":")[0],
		DstInterface: strings.Split(link.Endpoints[1], ":")[1],
	}
}
