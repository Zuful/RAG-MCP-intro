package embedders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/amikos-tech/chroma-go/pkg/embeddings"
)

// GemmaEmbeddingFunction est notre fonction d'embedding locale
type GemmaEmbeddingFunction struct {
	serverURL  string
	httpClient *http.Client
}

// On s'assure qu'elle implémente bien la même interface
var _ embeddings.EmbeddingFunction = (*GemmaEmbeddingFunction)(nil)

// NewGemmaEmbeddingFunction est le constructeur public
func NewGemmaEmbeddingFunction(url string) embeddings.EmbeddingFunction {
	return &GemmaEmbeddingFunction{
		serverURL:  url,
		httpClient: &http.Client{},
	}
}

type embedRequest struct {
	Texts []string `json:"texts"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (e *GemmaEmbeddingFunction) embed(texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(embedRequest{Texts: texts})
	if err != nil {
		return nil, fmt.Errorf("erreur JSON marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", e.serverURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("erreur création requête HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur appel serveur local: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("le serveur d'embedding a renvoyé une erreur (%s)", resp.Status)
	}

	var embedResp embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("erreur JSON decode: %w", err)
	}

	return embedResp.Embeddings, nil
}

// EmbedDocuments implémente l'interface
func (e *GemmaEmbeddingFunction) EmbedDocuments(ctx context.Context, docs []string) ([]embeddings.Embedding, error) {
	floatEmbeddings, err := e.embed(docs)
	if err != nil {
		return nil, err
	}

	results := make([]embeddings.Embedding, len(floatEmbeddings))
	for i, data := range floatEmbeddings {
		results[i] = embeddings.NewEmbeddingFromFloat32(data)
	}
	return results, nil
}

// EmbedQuery implémente l'interface
func (e *GemmaEmbeddingFunction) EmbedQuery(ctx context.Context, query string) (embeddings.Embedding, error) {
	floatEmbeddings, err := e.embed([]string{query})
	if err != nil {
		return nil, err
	}
	if len(floatEmbeddings) == 0 {
		return nil, fmt.Errorf("le serveur d'embedding n'a renvoyé aucun vecteur")
	}

	result := embeddings.NewEmbeddingFromFloat32(floatEmbeddings[0])
	return result, nil
}
