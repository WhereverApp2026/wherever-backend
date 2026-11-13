package votaciones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio; el service los traduce.
var (
	ErrNotFound  = errors.New("votación no encontrada")
	ErrDuplicate = errors.New("la parada ya tiene una votación abierta")
)

// Indexes son los índices de `votaciones`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{
				Keys: bson.D{{Key: "parada_id", Value: 1}},
				Options: options.Index().SetName("ux_parada_abierta").SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "estado", Value: StatusOpen}}),
			},
			{Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "creada_en", Value: -1}}, Options: options.Index().SetName("ix_grupo_creada")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "cierra_en", Value: 1}}, Options: options.Index().SetName("ix_estado_cierra")},
		},
	}}
}

// Repository lee y escribe `votaciones`. Cerrar es una sola operación condicionada a que
// siga abierta y con los mismos votos que se contaron: dos instancias no la cierran dos
// veces y ningún voto se pierde entre contar y cerrar.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Create guarda una votación nueva; ErrDuplicate si la parada ya tiene una abierta.
func (r *Repository) Create(ctx context.Context, v *Vote) error {
	_, err := r.col.InsertOne(ctx, v)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

// FindByID trae la votación.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Vote, error) {
	var v Vote
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Vote{}, ErrNotFound
	}
	return v, err
}

func (r *Repository) find(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]Vote, error) {
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Vote{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListByGroup trae las votaciones del grupo, de la más nueva a la más vieja (hasta 100).
func (r *Repository) ListByGroup(ctx context.Context, groupID bson.ObjectID) ([]Vote, error) {
	return r.find(ctx, bson.D{{Key: "grupo_id", Value: groupID}},
		options.Find().SetSort(bson.D{{Key: "creada_en", Value: -1}}).SetLimit(100))
}

// Cast guarda o reemplaza el voto de userID si la votación sigue abierta y no venció.
// Devuelve false si ya no se puede votar.
func (r *Repository) Cast(ctx context.Context, id, userID, choice bson.ObjectID, now time.Time) (bool, error) {
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: StatusOpen}, {Key: "cierra_en", Value: bson.D{{Key: "$gt", Value: now}}}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "votos." + userID.Hex(), Value: choice}, {Key: "actualizada_en", Value: now}}},
			{Key: "$inc", Value: bson.D{{Key: "votos_version", Value: 1}}},
		})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// Close cierra la votación con la ganadora contada sobre `counted`, solo si sigue abierta y
// nadie votó desde que se contó (misma `votos_version`). Devuelve false si otra petición la
// cerró o alguien votó en medio (quien llama vuelve a leer y a contar).
func (r *Repository) Close(ctx context.Context, counted Vote, winner bson.ObjectID, by string, now time.Time) (bool, error) {
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: counted.ID}, {Key: "estado", Value: StatusOpen}, {Key: "votos_version", Value: counted.BallotsVersion}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "estado", Value: StatusClosed}, {Key: "ganadora", Value: winner}, {Key: "cerrada_por", Value: by},
			{Key: "cerrada_en", Value: now}, {Key: "actualizada_en", Value: now},
		}}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// applyLease es cuánto dura la reserva para aplicar la ganadora; si la instancia se cae,
// otra la toma al vencer.
const applyLease = time.Minute

// ClaimApply reserva aplicar la ganadora de una votación cerrada y sin aplicar: solo una
// instancia la obtiene a la vez. Devuelve false si ya está aplicada o reservada.
func (r *Repository) ClaimApply(ctx context.Context, id bson.ObjectID, now time.Time) (bool, error) {
	res, err := r.col.UpdateOne(ctx, bson.D{
		{Key: "_id", Value: id}, {Key: "estado", Value: StatusClosed},
		{Key: "aplicada", Value: bson.D{{Key: "$exists", Value: false}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "aplicando_hasta", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "aplicando_hasta", Value: bson.D{{Key: "$lte", Value: now}}}},
		}},
	}, bson.D{{Key: "$set", Value: bson.D{{Key: "aplicando_hasta", Value: now.Add(applyLease)}}}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// SetApplied deja constancia de si la ganadora ya está en el plan y por qué no.
func (r *Repository) SetApplied(ctx context.Context, id bson.ObjectID, applied bool, reason string, now time.Time) error {
	set := bson.D{{Key: "aplicada", Value: applied}, {Key: "actualizada_en", Value: now}}
	if reason != "" {
		set = append(set, bson.E{Key: "motivo", Value: reason})
	}
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: set}, {Key: "$unset", Value: bson.D{{Key: "aplicando_hasta", Value: ""}}}})
	return err
}

// Due trae hasta limit votaciones por terminar: abiertas que ya vencieron y cerradas cuya
// ganadora aún no se aplicó (la instancia se cayó a la mitad). Usa ix_estado_cierra.
func (r *Repository) Due(ctx context.Context, now time.Time, limit int64) ([]Vote, error) {
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "estado", Value: StatusOpen}, {Key: "cierra_en", Value: bson.D{{Key: "$lte", Value: now}}}},
		bson.D{{Key: "estado", Value: StatusClosed}, {Key: "aplicada", Value: bson.D{{Key: "$exists", Value: false}}}},
	}}}
	return r.find(ctx, filter, options.Find().SetSort(bson.D{{Key: "cierra_en", Value: 1}}).SetLimit(limit))
}

// ListVotedBy trae las votaciones donde userID votó, para exportarlas.
func (r *Repository) ListVotedBy(ctx context.Context, userID bson.ObjectID) ([]Vote, error) {
	return r.find(ctx, bson.D{{Key: "votos." + userID.Hex(), Value: bson.D{{Key: "$exists", Value: true}}}},
		options.Find().SetSort(bson.D{{Key: "creada_en", Value: 1}}))
}

// DeleteByUser borra las votaciones que abrió userID y quita sus votos de las demás.
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.col.DeleteMany(ctx, bson.D{{Key: "creada_por", Value: userID}}); err != nil {
		return err
	}
	key := "votos." + userID.Hex()
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: key, Value: bson.D{{Key: "$exists", Value: true}}}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: key, Value: ""}}}})
	return err
}
