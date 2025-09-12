package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Charger les variables d'environnement depuis le fichier .env à la racine
	// Le chemin est relatif à l'emplacement où on exécute le binaire.
	// Pour un `go run` depuis la racine, c'est ".env"
	// Si on build et exécute depuis `cmd/novabot`, il faudrait `../../.env`
	// On va donc utiliser une approche plus robuste plus tard, mais pour ce test :
	err := godotenv.Load(".env")
	if err != nil {
		// Si on est dans un conteneur ou en prod, on n'aura pas de .env, c'est ok.
		// Mais pour notre dev local, c'est une erreur si on ne le trouve pas.
		log.Println("Attention: Fichier .env non trouvé. Utilisation des variables d'environnement système.")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("La variable d'environnement OPENAI_API_KEY est requise.")
	}

	chromaURL := os.Getenv("CHROMA_DB_URL")
	if chromaURL == "" {
		log.Fatal("La variable d'environnement CHROMA_DB_URL est requise.")
	}

	fmt.Println("✅ Configuration chargée avec succès !")
	fmt.Printf("   - Clé OpenAI trouvée (se termine par): ...%s\n", apiKey[len(apiKey)-4:])
	fmt.Printf("   - URL ChromaDB: %s\n", chromaURL)
}
