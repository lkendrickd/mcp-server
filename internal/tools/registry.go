package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Registrar is a function that registers tools with an MCP server.
type Registrar func(server *mcp.Server)

// Registry holds all tool registrars.
var Registry []Registrar

// Register adds a tool registrar to the registry.
func Register(r Registrar) {
	Registry = append(Registry, r)
}

// RegisterAll registers all tools with the given MCP server.
func RegisterAll(server *mcp.Server) {
	for _, r := range Registry {
		r(server)
	}
}
