package embeddings

import (
	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
)

// Armar construye el recálculo con Gemini detrás de los límites de IA_EMB_LIMITE_MIN/DIA. Sin
// GEMINI_API_KEY o IA_EMBEDDINGS_ID devuelve nil: no hay vectores y responde el motor de reglas.
// urlBase vacía usa la API pública; las pruebas pasan un servidor local.
func Armar(cfg config.IA, rdb *goredis.Client, db *mongo.Database, urlBase string) (*Recalculador, error) {
	if !cfg.EmbeddingsEnabled() {
		return nil, nil
	}
	g, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: cfg.GeminiAPIKey, ModeloID: cfg.EmbeddingsID, Timeout: cfg.Timeout, URLBase: urlBase})
	if err != nil {
		return nil, err
	}
	limitado, err := ia.NewVectoresLimitados(g, ia.NewLimitadorRedis(rdb), ia.LimitesVectores{PorMinuto: cfg.EmbLimiteMin, PorDia: cfg.EmbLimiteDia})
	if err != nil {
		return nil, err
	}
	return NewRecalculador(NewRepository(db), limitado, cfg.EmbLimiteMin)
}
