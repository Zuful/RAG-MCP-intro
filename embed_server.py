from flask import Flask, request, jsonify
from sentence_transformers import SentenceTransformer
import traceback

# 1. On charge le modèle une seule fois au démarrage.
# Note : Pour l'esprit de Gemma, on utilise un modèle très léger et performant.
# 'all-MiniLM-L6-v2' est un excellent choix pour commencer.
# La première fois, il sera téléchargé automatiquement (environ 90 Mo).
print("Chargement du modèle d'embedding...")
model = SentenceTransformer('google/embeddinggemma-300m')
print("Modèle chargé avec succès.")

app = Flask(__name__)

@app.route('/embed', methods=['POST'])
def embed():
    try:
        data = request.get_json()
        if not data or 'texts' not in data:
            return jsonify({"error": "Le champ 'texts' est manquant dans le JSON"}), 400

        texts = data['texts']
        print(f"Traitement de {len(texts)} textes, longueurs: {[len(t) for t in texts]}")
        
        # Générer les embeddings
        embeddings = model.encode(texts).tolist()
        
        # Nettoyer les valeurs NaN (remplacer par 0.0 pour assurer un JSON valide)
        import math
        for i, embedding in enumerate(embeddings):
            nan_count = sum(1 for x in embedding if math.isnan(x))
            if nan_count > 0:
                print(f"Attention: Embedding {i} contient {nan_count} valeurs NaN, remplacées par 0.0")
                embeddings[i] = [0.0 if math.isnan(x) else x for x in embedding]

        return jsonify({"embeddings": embeddings})

    except Exception as e:
        print(f"Erreur dans embed(): {str(e)}")
        print(f"Traceback: {traceback.format_exc()}")
        return jsonify({"error": f"Erreur de traitement: {str(e)}"}), 500

if __name__ == '__main__':
    # On lance le serveur sur le port 5000
    app.run(host='0.0.0.0', port=5001)