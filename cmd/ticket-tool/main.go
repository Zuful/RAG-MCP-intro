package main

import (
	// On importe le package mcp que nous venons de créer
	"github.com/Zuful/novabot/internal/mcp"
)

func main() {
	// On lance le serveur sur le port 8081 (pour ne pas entrer en conflit
	// avec ChromaDB qui est sur 8000).
	mcp.StartTicketToolServer("8081")
}
