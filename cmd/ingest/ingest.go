package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	// ON A BESOIN DE CE NOUVEAU PACKAGE POUR L'INTERFACE
	"github.com/amikos-tech/chroma-go/pkg/embeddings"
)

// Document struct
type Document struct {
	Text     string
	Metadata map[string]interface{}
}

func main() {
	// ... (Toutes les sections précédentes sont maintenant correctes)
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	chromaURL := os.Getenv("CHROMA_DB_URL")
	if openaiAPIKey == "" || chromaURL == "" {
		log.Fatal("OPENAI_API_KEY et CHROMA_DB_URL doivent être définis")
	}
	fmt.Println("🚀 Démarrage du script d'ingestion...")
	ctx := context.Background()
	docs, err := loadDocuments("./data")
	if err != nil {
		log.Fatalf("Erreur lecture documents: %v", err)
	}
	fmt.Printf("   - %d documents trouvés\n", len(docs))
	openaiClient := openai.NewClient(openaiAPIKey)
	chromaClient, err := chroma.NewHTTPClient(chroma.WithBaseURL(chromaURL))
	if err != nil {
		log.Fatalf("Erreur client Chroma: %v", err)
	}
	collectionName := "novabot-rh"
	fmt.Printf("   - Préparation de la collection Chroma '%s'...\n", collectionName)
	err = chromaClient.DeleteCollection(ctx, collectionName)
	if err != nil {
		log.Printf("Avertissement: collection non supprimée (elle n'existait peut-être pas): %v", err)
	}

	// Cette ligne est maintenant correcte car NewOpenAIEmbeddingFunction va retourner le bon type
	embeddingFunc := NewOpenAIEmbeddingFunction(openaiClient)

	col, err := chromaClient.GetOrCreateCollection(
		ctx,
		collectionName,
		chroma.WithEmbeddingFunctionCreate(embeddingFunc),
	)
	if err != nil {
		log.Fatalf("Erreur création collection: %v", err)
	}

	// ... (le reste de la fonction main est correct)
	fmt.Println("   - Vectorisation et ajout des documents...")
	texts := make([]string, len(docs))
	docMetadatas := make([]chroma.DocumentMetadata, len(docs))
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
		docMetadatas[i] = chroma.NewDocumentMetadata(attributes...)
	}
	err = col.Add(ctx, chroma.WithIDs(ids...), chroma.WithMetadatas(docMetadatas...), chroma.WithTexts(texts...))
	if err != nil {
		log.Fatalf("Erreur ajout documents: %v", err)
	}
	fmt.Println("✅ Ingestion terminée avec succès !")
}

// loadDocuments (inchangé)
func loadDocuments(dir string) ([]Document, error) {
	// ...
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
// # SECTION ENTIÈREMENT MISE À JOUR POUR SATISFAIRE LA BONNE INTERFACE #
// ##################################################################

type openaiEmbeddingFunction struct {
	apiClient *openai.Client
}

var _ embeddings.EmbeddingFunction = (*openaiEmbeddingFunction)(nil)

func NewOpenAIEmbeddingFunction(client *openai.Client) embeddings.EmbeddingFunction {
	return &openaiEmbeddingFunction{apiClient: client}
}

// La méthode retourne maintenant `[]embeddings.Embedding`
func (e *openaiEmbeddingFunction) EmbedDocuments(ctx context.Context, docs []string) ([]embeddings.Embedding, error) {
	fmt.Printf("   - Création d'embeddings pour %d documents...\n", len(docs))
	resp, err := e.apiClient.CreateEmbeddings(ctx, &openai.EmbeddingRequest{Input: docs, Model: openai.AdaEmbeddingV2})
	if err != nil {
		return nil, err
	}

	// On crée une slice du bon type de retour
	results := make([]embeddings.Embedding, len(resp.Data))
	for i, data := range resp.Data {
		// On convertit chaque `[]float32` en `embeddings.Embedding`
		results[i] = embeddings.NewEmbeddingFromFloat32(data.Embedding)
	}
	return results, nil
}

func (e *openaiEmbeddingFunction) EmbedQuery(ctx context.Context, query string) (embeddings.Embedding, error) {
	resp, err := e.apiClient.CreateEmbeddings(ctx, &openai.EmbeddingRequest{Input: []string{query}, Model: openai.AdaEmbeddingV2})
	if err != nil {
		// --- DÉBUT DE LA CORRECTION ---
		// La valeur "zéro" d'une interface est `nil`
		return nil, err
		// --- FIN DE LA CORRECTION ---
	}
	result := embeddings.NewEmbeddingFromFloat32(resp.Data[0].Embedding)
	return result, nil
}
