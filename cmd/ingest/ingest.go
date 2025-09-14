package main

import (
	"context"
	"fmt"
	embedders "github.com/Zuful/novabot/internal/embeddings"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings"
)

// Document struct
type Document struct {
	Text     string
	Metadata map[string]interface{}
}

func main() {
	// --- AJOUT : Un simple booléen pour choisir notre moteur d'embedding ---
	useGemmaLocal := true
	// ---------------------------------------------------------------------

	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	chromaURL := os.Getenv("CHROMA_DB_URL")
	// MODIFICATION : On ne quitte que si on a besoin de la clé OpenAI
	if !useGemmaLocal && openaiAPIKey == "" {
		log.Fatal("OPENAI_API_KEY doit être défini pour utiliser OpenAI")
	}
	if chromaURL == "" {
		log.Fatal("CHROMA_DB_URL doit être défini")
	}

	fmt.Println("🚀 Démarrage du script d'ingestion...")
	ctx := context.Background()
	docs, err := loadDocuments("./data")
	if err != nil {
		log.Fatalf("Erreur lecture documents: %v", err)
	}
	fmt.Printf("   - %d documents trouvés\n", len(docs))

	// MODIFICATION : On crée le client Chroma directement
	chromaClient, err := chroma.NewHTTPClient(chroma.WithBaseURL(chromaURL))
	if err != nil {
		log.Fatalf("Erreur client Chroma: %v", err)
	}

	// --- AJOUT : Logique pour sélectionner la fonction d'embedding ---
	var embeddingFunc embeddings.EmbeddingFunction

	if useGemmaLocal {
		fmt.Println("   - Utilisation du moteur d'embedding local (Gemma-like)")
		embeddingFunc = embedders.NewGemmaEmbeddingFunction("http://localhost:5001/embed")
	} else {
		fmt.Println("   - Utilisation du moteur d'embedding OpenAI")
		openaiClient := openai.NewClient(openaiAPIKey)
		embeddingFunc = NewOpenAIEmbeddingFunction(openaiClient)
	}
	// -------------------------------------------------------------

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

	fmt.Println("   - Vectorisation et ajout des documents...")

	// MODIFICATION : Correction des types pour l'appel Add
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

// loadDocuments (inchangé, c'est votre version)
func loadDocuments(dir string) ([]Document, error) {
	var documents []Document
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, _ := os.ReadFile(path)
			doc := Document{
				Text:     string(content),
				Metadata: map[string]interface{}{"source": info.Name()},
			}
			documents = append(documents, doc)
		}
		return nil
	})
	return documents, nil
}

// ##################################################################
// # SECTION OPENAI (INCHANGÉE, C'EST VOTRE VERSION)                #
// ##################################################################

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
