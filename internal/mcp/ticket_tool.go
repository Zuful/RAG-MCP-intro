package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// La structure de la requête que notre outil attend
type CreateTicketRequest struct {
	User  string `json:"user"`
	Query string `json:"query"`
}

// StartTicketToolServer lance notre petit serveur MCP sur un port donné.
func StartTicketToolServer(port string) {
	// On définit une "route" : quand une requête POST arrive sur /create-ticket,
	// on appelle la fonction handleCreateTicket.
	http.HandleFunc("/create-ticket", handleCreateTicket)

	fmt.Printf("MCP Tool 'HR-Ticket-Tool' démarré et en écoute sur le port %s\n", port)

	// On lance le serveur. La fonction est bloquante, elle tournera indéfiniment.
	// On utilise log.Fatal pour que le programme s'arrête si le serveur crashe.
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// handleCreateTicket est la fonction qui contient la logique de notre outil.
func handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	// 1. On s'assure que la requête est bien de type POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// 2. On décode le corps de la requête JSON dans notre structure
	var req CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Requête JSON invalide", http.StatusBadRequest)
		return
	}

	// 3. On simule la création du ticket en l'affichant dans la console
	// Dans un vrai projet, on appellerait ici l'API de Jira, Zendesk, etc.
	log.Printf("[MCP TOOL] ✅ Nouveau ticket créé pour l'utilisateur '%s' concernant la question : '%s'\n", req.User, req.Query)

	// 4. On renvoie une réponse de succès au client (notre futur NovaBot)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ticket créé avec succès"))
}
