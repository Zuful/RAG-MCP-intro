package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// --- CONFIGURATION DU CERVEAU N°2 ---
// Mettez à `false` pour utiliser l'API OpenAI payante
const useOllamaLocal = true

// ------------------------------------

// --- VARIABLES GLOBALES (mise à jour pour Qdrant) ---
var qdrantURL string
var ollamaURL string
var httpClient *http.Client
var embeddingServiceURL string
var openaiClient *openai.Client // Gardé pour l'option OpenAI

const collectionName = "novabot-rh"

// setupClients (mis à jour pour Qdrant)
func setupClients() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}
	qdrantURL = os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		qdrantURL = "http://localhost:6333" // Valeur par défaut
	}

	ollamaURL = os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434" // Valeur par défaut
	}

	// Initialiser le client HTTP
	httpClient = &http.Client{Timeout: 30 * time.Second}

	if !useOllamaLocal {
		openaiAPIKey := os.Getenv("OPENAI_API_KEY")
		if openaiAPIKey == "" {
			log.Fatal("OPENAI_API_KEY doit être défini pour utiliser OpenAI")
		}
		openaiClient = openai.NewClient(openaiAPIKey)
	}

	// Configurer l'URL du service d'embedding
	embeddingServiceURL = "http://localhost:5001/embed"

	// Vérifier que la collection existe dans Qdrant
	if err := checkQdrantCollection(); err != nil {
		log.Fatalf("Erreur pour vérifier la collection '%s' dans Qdrant: %v.\nAvez-vous bien lancé le script d'ingestion en premier ?", collectionName, err)
	}
	fmt.Println("✅ Clients et connexion à Qdrant prêts.")
}

// checkQdrantCollection vérifie que la collection existe dans Qdrant
func checkQdrantCollection() error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/collections/%s", qdrantURL, collectionName), nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("collection '%s' n'existe pas dans Qdrant (status: %d)", collectionName, resp.StatusCode)
	}

	return nil
}

// QdrantSearchRequest représente une requête de recherche Qdrant avec support de filtrage
type QdrantSearchRequest struct {
	Vector      []float32              `json:"vector"`
	Limit       int                    `json:"limit"`
	WithPayload bool                   `json:"with_payload"`
	Filter      map[string]interface{} `json:"filter,omitempty"`
}

// QdrantSearchResult représente le résultat d'une recherche Qdrant
type QdrantSearchResult struct {
	Result []struct {
		ID      string                 `json:"id"`
		Score   float64                `json:"score"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
}

// generateEmbedding appelle le service d'embedding pour générer un embedding
func generateEmbedding(text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string][]string{"texts": {text}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", embeddingServiceURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erreur service embedding: status %d", resp.StatusCode)
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("aucun embedding généré")
	}

	return result.Embeddings[0], nil
}

// getAvailableTopics récupère tous les topics disponibles dans la collection
func getAvailableTopics() ([]string, error) {
	// Récupérer un échantillon de documents pour extraire les topics
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/collections/%s/points/scroll", qdrantURL, collectionName), bytes.NewBuffer([]byte(`{"limit": 100, "with_payload": true}`)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Points []struct {
				ID      string                 `json:"id"`
				Payload map[string]interface{} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Extraire les topics uniques
	topicsMap := make(map[string]bool)
	for _, point := range result.Result.Points {
		if topic, ok := point.Payload["topic"].(string); ok && topic != "" {
			topicsMap[topic] = true
		}
	}

	// Convertir en slice
	topics := make([]string, 0, len(topicsMap))
	for topic := range topicsMap {
		topics = append(topics, topic)
	}

	return topics, nil
}

// analyzeQueryWithLLM utilise Gemma 3 pour analyser la requête et identifier les topics pertinents
func analyzeQueryWithLLM(query string, availableTopics []string) ([]string, error) {
	if len(availableTopics) == 0 {
		return nil, nil // Pas de filtrage si pas de topics
	}

	// Créer le prompt pour l'analyse de topic
	topicsStr := strings.Join(availableTopics, ", ")
	analysisPrompt := fmt.Sprintf(`Analyze this user query and determine which topics are most relevant.

User Query: "%s"

Available Topics: %s

Instructions:
- Return ONLY the most relevant topic names from the available topics
- If multiple topics are relevant, separate them with commas
- If no topics are clearly relevant, return "none"
- Be precise and only include topics that directly relate to the query

Relevant topics:`, query, topicsStr)

	// Debug: afficher les topics disponibles
	fmt.Printf("[DEBUG LLM] Topics disponibles: %v\n", availableTopics)
	fmt.Printf("[DEBUG LLM] Requête: %s\n", query)

	// Appeler Ollama pour l'analyse
	reqBody := map[string]interface{}{
		"model":  "gemma3:12b", // Utiliser le modèle disponible
		"prompt": analysisPrompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.1, // Faible température pour plus de précision
			"num_predict": 50,   // Limite la réponse
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ollamaURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Parser la réponse
	response := strings.TrimSpace(result.Response)
	fmt.Printf("[DEBUG LLM] Réponse du LLM: '%s'\n", response)
	if response == "none" || response == "" {
		return nil, nil
	}

	// Séparer les topics multiples et nettoyer
	relevantTopics := []string{}
	for _, topic := range strings.Split(response, ",") {
		topic = strings.TrimSpace(topic)
		// Vérifier que le topic existe dans la liste disponible
		for _, availableTopic := range availableTopics {
			if strings.EqualFold(topic, availableTopic) {
				relevantTopics = append(relevantTopics, availableTopic)
				break
			}
		}
	}

	return relevantTopics, nil
}

// createTopicFilter crée un filtre Qdrant basé sur les topics sélectionnés par le LLM
func createTopicFilter(topics []string) map[string]interface{} {
	if len(topics) == 0 {
		return nil
	}

	// Si un seul topic
	if len(topics) == 1 {
		return map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "topic",
					"match": map[string]interface{}{
						"value": topics[0],
					},
				},
			},
		}
	}

	// Si plusieurs topics, utiliser "should" (OR logic)
	shouldFilters := []map[string]interface{}{}
	for _, topic := range topics {
		shouldFilters = append(shouldFilters, map[string]interface{}{
			"key": "topic",
			"match": map[string]interface{}{
				"value": topic,
			},
		})
	}

	return map[string]interface{}{
		"should": shouldFilters,
	}
}

func searchQdrant(query string, limit int) ([]string, []map[string]interface{}, error) {
	// 1. Récupérer les topics disponibles
	availableTopics, err := getAvailableTopics()
	if err != nil {
		fmt.Printf("[WARNING] Impossible de récupérer les topics: %v\n", err)
	}

	// 2. Analyser la requête avec le LLM pour identifier les topics pertinents
	relevantTopics, err := analyzeQueryWithLLM(query, availableTopics)
	if err != nil {
		fmt.Printf("[WARNING] Erreur d'analyse LLM: %v\n", err)
	}

	// 3. Générer l'embedding pour la requête
	embedding, err := generateEmbedding(query)
	if err != nil {
		return nil, nil, fmt.Errorf("erreur génération embedding: %w", err)
	}

	// 4. Créer le filtre basé sur les topics identifiés
	topicFilter := createTopicFilter(relevantTopics)

	// 5. Debug: afficher les informations de filtrage
	if len(relevantTopics) > 0 {
		fmt.Printf("[DEBUG TOPIC] Topics pertinents détectés: %v\n", relevantTopics)
	} else {
		fmt.Printf("[DEBUG TOPIC] Aucun filtrage par topic\n")
	}

	// 6. Préparer la requête de recherche avec filtrage
	searchReq := QdrantSearchRequest{
		Vector:      embedding,
		Limit:       limit,
		WithPayload: true,
		Filter:      topicFilter,
	}

	jsonData, err := json.Marshal(searchReq)
	if err != nil {
		return nil, nil, err
	}

	// Debug: Log the search request
	fmt.Printf("[DEBUG SEARCH] Query: %s\n", query)
	fmt.Printf("[DEBUG SEARCH] Request: %s\n", string(jsonData)[:200])

	// Effectuer la recherche
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/collections/%s/points/search", qdrantURL, collectionName), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("erreur recherche Qdrant: status %d", resp.StatusCode)
	}

	var result QdrantSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	// Extraire les textes et métadonnées
	texts := make([]string, len(result.Result))
	metadatas := make([]map[string]interface{}, len(result.Result))

	for i, point := range result.Result {
		if text, ok := point.Payload["text"].(string); ok {
			texts[i] = text
		} else {
			texts[i] = "" // Texte vide si pas trouvé
		}
		metadatas[i] = point.Payload
	}

	return texts, metadatas, nil
}

func main() {
	setupClients()
	if useOllamaLocal {
		fmt.Println("--- NovaBot, votre assistant RH (Mode 100% Local) ---")
	} else {
		fmt.Println("--- NovaBot, votre assistant RH (Mode Hybride) ---")
	}
	fmt.Println("Posez vos questions ou tapez 'quitter' pour arrêter.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nVous: ")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if strings.ToLower(userInput) == "quitter" {
			break
		}

		// Phase RAG (mis à jour pour Qdrant)
		fmt.Println("NovaBot pense...")
		documents, metadatas, err := searchQdrant(userInput, 20) // Increase to capture lower-ranking relevant content
		if err != nil {
			log.Printf("Erreur de recherche RAG: %v", err)
			continue
		}
		
		// Debug temporaire pour Oselia
		if strings.Contains(strings.ToLower(userInput), "oselia") {
			fmt.Printf("[DEBUG OSELIA] Documents trouvés: %d\n", len(documents))
			for i, meta := range metadatas {
				if meta != nil {
					fmt.Printf("[DEBUG] Doc %d - Source: %v\n", i+1, meta["source"])
				}
			}
		}
		
		// Debug temporaire pour dotgo
		if strings.Contains(strings.ToLower(userInput), "dotgo") {
			fmt.Printf("[DEBUG DOTGO] Documents trouvés: %d\n", len(documents))
			for i, meta := range metadatas {
				if meta != nil {
					fmt.Printf("[DEBUG] Doc %d - Source: %v\n", i+1, meta["source"])
				}
			}
		}

		var contextBuilder strings.Builder
		if len(documents) > 0 {
			for i, docText := range documents {
				source := "Source inconnue"
				if i < len(metadatas) && metadatas[i] != nil {
					if val, ok := metadatas[i]["source"].(string); ok {
						source = val
					}
				}
				contextBuilder.WriteString(fmt.Sprintf("\n---\nExtrait de document %d (source: %s):\n%s\n---\n", i+1, source, docText))
			}
		} else {
			contextBuilder.WriteString("\n---\nAucun document trouvé dans la base de connaissance.\n---\n")
		}

		// --- MODIFICATION : SÉLECTION DU CERVEAU N°2 ---
		var llmResponse string
		if useOllamaLocal {
			llmResponse, err = callOllama(userInput, contextBuilder.String())
		} else {
			llmResponse, err = callOpenAI(userInput, contextBuilder.String())
		}

		if err != nil {
			log.Printf("Erreur du Cerveau n°2: %v", err)
			continue
		}
		// --- SECTION DE PARSING AMÉLIORÉE ---
		fmt.Print("NovaBot: ")
		if strings.TrimSpace(llmResponse) == "TICKET" {
			fmt.Println("Je ne peux pas répondre à cette question. Je crée un ticket pour que l'équipe RH vous recontacte.")

			// On utilise des valeurs simples car le LLM ne les fournit plus
			jsonArgs := fmt.Sprintf(`{"user": "Jean", "query": "%s"}`, strings.ReplaceAll(userInput, `"`, `\"`))

			err := callCreateTicketTool(jsonArgs)
			if err != nil {
				log.Printf("Erreur lors de l'appel à l'outil MCP: %v", err)
			}
		} else {
			fmt.Println(llmResponse)
		}
		// ----------------------------------------------------
	}
}

// --- AJOUT : FONCTION POUR APPELER OLLAMA ---
func callOllama(userInput, ragContext string) (string, error) {
	systemPrompt := `Tu es un assistant expert en extraction de réponse.
	Ta seule tâche est de répondre à la question de l'utilisateur en te basant sur le contexte fourni.
	- Si le contexte contient la réponse, formule une réponse courte et directe.
	- Si le contexte ne contient PAS la réponse, ou si la question est personnelle, réponds UNIQUEMENT avec le mot : "TICKET". Ne dis rien d'autre.`

	userMessage := fmt.Sprintf("Contexte: %s\n\nQuestion: %s", ragContext, userInput)

	type ollamaRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		System string `json:"system"`
		Stream bool   `json:"stream"`
	}
	type ollamaResponse struct {
		Response string `json:"response"`
	}

	reqBody, _ := json.Marshal(ollamaRequest{
		Model:  "gemma3:12b",
		System: systemPrompt,
		Prompt: userMessage, // On utilise "Prompt" au lieu de "Messages" pour l'API /api/generate
		Stream: false,
	})

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("impossible de contacter le serveur Ollama. Est-il bien lancé ? (%w)", err)
	}
	defer resp.Body.Close()

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", err
	}

	return strings.TrimSpace(ollamaResp.Response), nil
}

// ------------------------------------------

// --- MODIFICATION : LA FONCTION OPENAI EXISTANTE ---
func callOpenAI(userInput, ragContext string) (string, error) {
	systemPrompt := `Tu es NovaBot, un assistant RH.
- Réponds aux questions en te basant EXCLUSIVEMENT sur le contexte fourni.
- Si le contexte ne te permet pas de répondre, ou si la question est personnelle, tu DOIS utiliser l'outil 'create_ticket'.
- Sois bref et direct.`

	tool := openai.Tool{ /* ... (votre code de l'outil est parfait) ... */ }
	userMessage := fmt.Sprintf("Contexte: %s\n\nQuestion de l'employé 'Jean': %s", ragContext, userInput)

	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userMessage},
			},
			Tools: []openai.Tool{tool},
		},
	)
	if err != nil {
		return "", err
	}

	message := resp.Choices[0].Message
	if len(message.ToolCalls) > 0 {
		toolCall := message.ToolCalls[0]
		if toolCall.Function.Name == "create_ticket" {
			// On extrait les arguments du JSON renvoyé par OpenAI
			var args map[string]string
			json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

			// On formate la sortie pour qu'elle ressemble à celle d'Ollama
			// C'est ici qu'on utilise notre nouveau séparateur "::"
			return fmt.Sprintf("TOOL_CALL::create_ticket::%s::%s", args["user"], args["query"]), nil
		}
	}

	return message.Content, nil
}

// -----------------------------------------------

// callCreateTicketTool (votre code, parfait et inchangé)
func callCreateTicketTool(arguments string) error {
	reqBody := bytes.NewBuffer([]byte(arguments))
	resp, err := http.Post("http://localhost:8083/create-ticket", "application/json", reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("l'outil MCP a renvoyé un statut d'erreur: %s", resp.Status)
	}
	return nil
}

// Helper function for debugging
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
