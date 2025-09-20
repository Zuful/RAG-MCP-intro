package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
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

// Structure pour l'embedding service
type EmbeddingRequest struct {
	Texts []string `json:"texts"`
}

type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Structures pour l'embeddingestion service
type VectorDocument struct {
	ID       string                 `json:"id"`
	Vectors  []float32             `json:"vectors"`
	Text     string                `json:"text,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type StoreVectorsRequest struct {
	CollectionName string            `json:"collection_name"`
	Documents      []VectorDocument  `json:"documents"`
}

type StorageResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	DocumentsCount int    `json:"documents_count,omitempty"`
	CollectionName string `json:"collection_name,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ------------------------------------------------------------------
// SEULE CETTE FONCTION EST REMPLACÉE
// ------------------------------------------------------------------
// loadDocuments appelle maintenant le microservice docparser.
func loadDocuments(dir string, parserURL string) ([]Document, error) {
	var documents []Document
	client := &http.Client{Timeout: 60 * time.Second} // 1 minute for document parsing

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

// callEmbeddingService génère les embeddings via le service d'embedding avec traitement par batches
func callEmbeddingService(texts []string, embeddingURL string) ([][]float32, error) {
	client := &http.Client{Timeout: 60 * time.Second} // 1 minute per batch
	batchSize := 3 // Process 3 documents at a time

	fmt.Printf("   - Génération des embeddings pour %d documents (par batches de %d)...\n", len(texts), batchSize)

	var allEmbeddings [][]float32

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		batchNum := (i / batchSize) + 1
		totalBatches := (len(texts) + batchSize - 1) / batchSize

		fmt.Printf("     > Traitement du batch %d/%d (%d documents)...\n", batchNum, totalBatches, len(batch))

		// Process this batch
		reqBody, err := json.Marshal(EmbeddingRequest{Texts: batch})
		if err != nil {
			return nil, fmt.Errorf("erreur marshalling JSON batch %d: %w", batchNum, err)
		}

		req, err := http.NewRequest("POST", embeddingURL, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("erreur création requête batch %d: %w", batchNum, err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("erreur appel service d'embedding batch %d: %w", batchNum, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("service d'embedding a retourné une erreur (%s) pour batch %d", resp.Status, batchNum)
		}

		var embeddingResp EmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
			return nil, fmt.Errorf("erreur décodage réponse batch %d: %w", batchNum, err)
		}

		// Add embeddings from this batch to our results
		allEmbeddings = append(allEmbeddings, embeddingResp.Embeddings...)
		fmt.Printf("     ✅ Batch %d/%d terminé (%d embeddings générés)\n", batchNum, totalBatches, len(embeddingResp.Embeddings))
	}

	fmt.Printf("   ✅ Tous les embeddings générés avec succès (%d documents total)\n", len(allEmbeddings))
	return allEmbeddings, nil
}

// storeVectors stocke les vecteurs via l'embeddingestion service
func storeVectors(documents []VectorDocument, collectionName, embeddingestionURL string) error {
	client := &http.Client{Timeout: 60 * time.Second}

	fmt.Printf("   - Stockage de %d documents vectorisés dans la collection '%s'...\n", len(documents), collectionName)

	reqBody, err := json.Marshal(StoreVectorsRequest{
		CollectionName: collectionName,
		Documents:      documents,
	})
	if err != nil {
		return fmt.Errorf("erreur marshalling JSON: %w", err)
	}

	req, err := http.NewRequest("POST", embeddingestionURL+"/api/v1/vectors", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("erreur création requête: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erreur appel embeddingestion service: %w", err)
	}
	defer resp.Body.Close()

	var storageResp StorageResponse
	if err := json.NewDecoder(resp.Body).Decode(&storageResp); err != nil {
		return fmt.Errorf("erreur décodage réponse: %w", err)
	}

	if !storageResp.Success {
		return fmt.Errorf("embeddingestion service a retourné une erreur: %s", storageResp.Error)
	}

	fmt.Printf("   - Stockage réussi: %s (%d documents)\n", storageResp.Message, storageResp.DocumentsCount)
	return nil
}

// ------------------------------------------------------------------

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Erreur chargement .env: %v", err)
	}

	// Configuration des URLs des microservices
	docParserURL := getEnvWithDefault("DOC_PARSER_URL", "http://localhost:8080/parse")
	embeddingURL := getEnvWithDefault("EMBEDDING_URL", "http://localhost:5001/embed")
	embeddingestionURL := getEnvWithDefault("EMBEDDINGESTION_URL", "http://localhost:8081")
	collectionName := getEnvWithDefault("COLLECTION_NAME", "novabot-rh")

	fmt.Println("🚀 Démarrage de l'orchestrateur d'ingestion...")
	fmt.Println("   - DocParser:", docParserURL)
	fmt.Println("   - Embedding Service:", embeddingURL)
	fmt.Println("   - Embeddingestion Service:", embeddingestionURL)
	fmt.Println("   - Collection:", collectionName)

	// ÉTAPE 1: Parser les documents via DocParser
	fmt.Println("\n📄 ÉTAPE 1: Parsing des documents...")
	docs, err := loadDocuments("./data", docParserURL)
	if err != nil {
		log.Fatalf("Erreur lors du parsing: %v", err)
	}

	if len(docs) == 0 {
		log.Fatal("Aucun document traité. Vérifiez que DocParser est lancé et que /data contient des fichiers.")
	}
	fmt.Printf("   ✅ %d documents parsés avec succès\n", len(docs))

	// ÉTAPE 2: Générer les embeddings via le service d'embedding
	fmt.Println("\n🧠 ÉTAPE 2: Génération des embeddings...")
	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Text
	}

	embeddings, err := callEmbeddingService(texts, embeddingURL)
	if err != nil {
		log.Fatalf("Erreur lors de la génération des embeddings: %v", err)
	}
	fmt.Printf("   ✅ Embeddings générés pour %d documents\n", len(embeddings))

	// ÉTAPE 3: Préparer les VectorDocuments pour le stockage
	fmt.Println("\n💾 ÉTAPE 3: Préparation des documents vectorisés...")
	vectorDocs := make([]VectorDocument, len(docs))
	for i, doc := range docs {
		vectorDocs[i] = VectorDocument{
			ID:       fmt.Sprintf("doc_%d_%d", time.Now().Unix(), i),
			Vectors:  embeddings[i],
			Text:     doc.Text,
			Metadata: doc.Metadata,
		}
	}
	fmt.Printf("   ✅ %d documents vectorisés prêts pour le stockage\n", len(vectorDocs))

	// ÉTAPE 4: Stocker via l'embeddingestion service
	fmt.Println("\n📍 ÉTAPE 4: Stockage des vecteurs...")
	err = storeVectors(vectorDocs, collectionName, embeddingestionURL)
	if err != nil {
		log.Fatalf("Erreur lors du stockage: %v", err)
	}

	fmt.Println("\n✅ Orchestration terminée avec succès !")
	fmt.Printf("   - %d documents traités et stockés dans la collection '%s'\n", len(docs), collectionName)
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

