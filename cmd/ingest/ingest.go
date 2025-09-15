package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	// Votre import est correct
	embedders "github.com/Zuful/novabot/internal/embeddings"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// La structure Document ne change pas
type Document struct {
	Text     string
	Metadata map[string]interface{}
}

// La structure de la réponse que nous attendons de notre service DocParser
type DocParserResponse struct {
	Content string `json:"content"`
}

// ------------------------------------------------------------------
// SEULE CETTE FONCTION EST REMPLACÉE
// ------------------------------------------------------------------
// loadDocuments appelle maintenant le microservice docparser.
func loadDocuments(dir string, parserURL string) ([]Document, error) {
	var documents []Document
	client := &http.Client{}

	fmt.Printf("   - Recherche de documents dans '%s' pour parsing via DocParser...\n", dir)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		fmt.Printf("      > Envoi du fichier '%s' au DocParser...\n", info.Name())

		// 1. Ouvrir le fichier local
		file, err := os.Open(path)
		if err != nil {
			log.Printf("      ! AVERTISSEMENT: Impossible d'ouvrir le fichier %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}
		defer file.Close()

		// 2. Préparer le corps de la requête HTTP (multipart/form-data)
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		part, err := writer.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			log.Printf("      ! AVERTISSEMENT: Impossible de préparer la requête pour %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}
		_, err = io.Copy(part, file)
		if err != nil {
			log.Printf("      ! AVERTISSEMENT: Impossible de lire le contenu de %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}
		writer.Close()

		// 3. Envoyer la requête au service DocParser
		req, err := http.NewRequest("POST", parserURL, &requestBody)
		if err != nil {
			log.Printf("      ! AVERTISSEMENT: Impossible de créer la requête HTTP pour %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("      ! AVERTISSEMENT: Echec de la connexion au DocParser pour %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("      ! AVERTISSEMENT: Le DocParser a renvoyé une erreur (%s) pour le fichier %s, ignoré.", resp.Status, info.Name())
			return nil
		}

		// 4. Décoder la réponse JSON et créer notre struct Document
		var parsedResponse DocParserResponse
		if err := json.NewDecoder(resp.Body).Decode(&parsedResponse); err != nil {
			log.Printf("      ! AVERTISSEMENT: Réponse invalide du DocParser pour %s, ignoré. Erreur: %v", info.Name(), err)
			return nil
		}

		doc := Document{
			Text:     parsedResponse.Content,
			Metadata: map[string]interface{}{"source": info.Name()},
		}
		documents = append(documents, doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des fichiers: %w", err)
	}

	return documents, nil
}

// ------------------------------------------------------------------

func main() {
	useGemmaLocal := true // Gardé pour la cohérence, même si OpenAI n'est plus utilisé ici

	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}
	chromaURL := os.Getenv("CHROMA_DB_URL")
	if chromaURL == "" {
		log.Fatal("CHROMA_DB_URL doit être défini")
	}

	fmt.Println("🚀 Démarrage du script d'ingestion...")
	ctx := context.Background()

	// --- MODIFICATION : On appelle la nouvelle fonction loadDocuments ---
	docs, err := loadDocuments("./data", "http://localhost:8080/parse")
	if err != nil {
		log.Fatalf("Le script d'ingestion a échoué: %v", err)
	}
	// -------------------------------------------------------------------

	if len(docs) == 0 {
		log.Fatal("Aucun document n'a pu être traité. Vérifiez que le service DocParser est bien lancé et que le dossier /data contient des fichiers valides.")
	}
	fmt.Printf("   - %d documents ont été traités avec succès par DocParser.\n", len(docs))

	// Le reste de votre code est 100% identique à votre version
	chromaClient, err := chroma.NewHTTPClient(chroma.WithBaseURL(chromaURL))
	if err != nil {
		log.Fatalf("Erreur client Chroma: %v", err)
	}

	var embeddingFunc embeddings.EmbeddingFunction
	if useGemmaLocal {
		fmt.Println("   - Utilisation du moteur d'embedding local (Gemma-like)")
		embeddingFunc = embedders.NewGemmaEmbeddingFunction("http://localhost:5001/embed")
	} else {
		fmt.Println("   - Utilisation du moteur d'embedding OpenAI")
		openaiAPIKey := os.Getenv("OPENAI_API_KEY")
		if openaiAPIKey == "" {
			log.Fatal("OPENAI_API_KEY doit être défini pour utiliser OpenAI")
		}
		openaiClient := openai.NewClient(openaiAPIKey)
		embeddingFunc = NewOpenAIEmbeddingFunction(openaiClient)
	}

	collectionName := "novabot-rh"
	fmt.Printf("   - Préparation de la collection Chroma '%s'...\n", collectionName)
	err = chromaClient.DeleteCollection(ctx, collectionName)
	if err != nil {
		log.Printf("Avertissement: collection non supprimée (elle n'existait peut-être pas): %v", err)
	}

	col, err := chromaClient.GetOrCreateCollection(
		ctx,
		collectionName,
		chroma.WithEmbeddingFunctionCreate(embeddingFunc),
	)
	if err != nil {
		log.Fatalf("Erreur création collection: %v", err)
	}

	fmt.Println("   - Vectorisation et ajout des documents à ChromaDB...")
	texts := make([]string, len(docs))
	metadatas := make([]chroma.DocumentMetadata, len(docs))
	ids := make([]chroma.DocumentID, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Text
		ids[i] = chroma.DocumentID(fmt.Sprintf("doc_%d", i))
		attributes := make([]*chroma.MetaAttribute, 0, len(doc.Metadata))
		for key, value := range doc.Metadata {
			if v, ok := value.(string); ok {
				attributes = append(attributes, chroma.NewStringAttribute(key, v))
			}
		}
		metadatas[i] = chroma.NewDocumentMetadata(attributes...)
	}

	err = col.Add(ctx, chroma.WithIDs(ids...), chroma.WithMetadatas(metadatas...), chroma.WithTexts(texts...))
	if err != nil {
		log.Fatalf("Erreur ajout documents: %v", err)
	}
	fmt.Println("✅ Ingestion terminée avec succès !")
}

// La section OpenAI est conservée car votre code la référence, même si elle n'est pas
// utilisée quand `useGemmaLocal` est `true`.
type openaiEmbeddingFunction struct {
	apiClient *openai.Client
}

var _ embeddings.EmbeddingFunction = (*openaiEmbeddingFunction)(nil)

func NewOpenAIEmbeddingFunction(client *openai.Client) embeddings.EmbeddingFunction {
	return &openaiEmbeddingFunction{apiClient: client}
}
func (e *openaiEmbeddingFunction) EmbedDocuments(ctx context.Context, docs []string) ([]embeddings.Embedding, error) {
	fmt.Printf("   - [OpenAI] Création d'embeddings pour %d documents...\n", len(docs))
	resp, err := e.apiClient.CreateEmbeddings(ctx, &openai.EmbeddingRequest{Input: docs, Model: openai.AdaEmbeddingV2})
	if err != nil {
		return nil, err
	}
	results := make([]embeddings.Embedding, len(resp.Data))
	for i, data := range resp.Data {
		results[i] = embeddings.NewEmbeddingFromFloat32(data.Embedding)
	}
	return results, nil
}
func (e *openaiEmbeddingFunction) EmbedQuery(ctx context.Context, query string) (embeddings.Embedding, error) {
	resp, err := e.apiClient.CreateEmbeddings(ctx, &openai.EmbeddingRequest{Input: []string{query}, Model: openai.AdaEmbeddingV2})
	if err != nil {
		return nil, err
	}
	result := embeddings.NewEmbeddingFromFloat32(resp.Data[0].Embedding)
	return result, nil
}
