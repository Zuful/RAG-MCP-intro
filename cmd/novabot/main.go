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

	// J'ai renommé votre import pour que ce soit plus clair et correspondre au dossier
	embedders "github.com/Zuful/novabot/internal/embeddings"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// --- CONFIGURATION DU CERVEAU N°2 ---
// Mettez à `false` pour utiliser l'API OpenAI payante
const useOllamaLocal = true

// ------------------------------------

// --- VARIABLES GLOBALES (votre version) ---
var chromaClient *chroma.Client
var openaiClient *openai.Client // Gardé pour l'option OpenAI
var collection chroma.Collection

const collectionName = "novabot-rh"

// setupClients (légèrement modifié pour ne charger la clé OpenAI que si nécessaire)
func setupClients() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}
	chromaURL := os.Getenv("CHROMA_DB_URL")
	if chromaURL == "" {
		log.Fatal("CHROMA_DB_URL doit être défini")
	}

	var err error
	chromaClient, err := chroma.NewHTTPClient(chroma.WithBaseURL(chromaURL))
	if err != nil {
		log.Fatalf("Erreur client Chroma: %v", err)
	}

	if !useOllamaLocal {
		openaiAPIKey := os.Getenv("OPENAI_API_KEY")
		if openaiAPIKey == "" {
			log.Fatal("OPENAI_API_KEY doit être défini pour utiliser OpenAI")
		}
		openaiClient = openai.NewClient(openaiAPIKey)
	}

	embeddingFunc := embedders.NewGemmaEmbeddingFunction("http://localhost:5001/embed")

	collection, err = chromaClient.GetCollection(
		context.Background(),
		collectionName,
		chroma.WithEmbeddingFunctionGet(embeddingFunc),
	)
	if err != nil {
		log.Fatalf("Erreur pour récupérer la collection '%s': %v.\nAvez-vous bien lancé le script d'ingestion en premier ?", collectionName, err)
	}
	fmt.Println("✅ Clients et connexion à la base de connaissances prêts.")
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

		// Phase RAG (votre code, parfait)
		fmt.Println("NovaBot pense...")
		results, err := collection.Query(
			context.Background(),
			chroma.WithQueryTexts(userInput),
			chroma.WithNResults(2),
		)
		if err != nil {
			log.Printf("Erreur de recherche RAG: %v", err)
			continue
		}

		var contextBuilder strings.Builder
		if len(results.GetDocumentsGroups()) > 0 && len(results.GetDocumentsGroups()[0]) > 0 {
			documents := results.GetDocumentsGroups()[0]
			metadatas := results.GetMetadatasGroups()[0]
			for i, docText := range documents {
				source := "Source inconnue"
				if i < len(metadatas) {
					metadataObject := metadatas[i]
					if val, ok := metadataObject.GetString("source"); ok {
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
