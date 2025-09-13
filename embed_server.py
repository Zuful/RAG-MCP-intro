from flask import Flask, request, jsonify
from sentence_transformers import SentenceTransformer

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

        # 2. On utilise le modèle pour encoder les textes en vecteurs
        embeddings = model.encode(texts).tolist() # .tolist() pour convertir en liste JSON-friendly

        return jsonify({"embeddings": embeddings})

    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    # On lance le serveur sur le port 5000
    app.run(host='0.0.0.0', port=5001)