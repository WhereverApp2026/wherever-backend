package consejos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrOcupado: otra petición ya tiene el documento del día (ux_consejo_dia).
var ErrOcupado = errors.New("consejos: el consejo del día ya existe")

// Repository lee y escribe `consejos_negocio` y lee, por nombre, los agregados de nv-61, nv-62 y
// nv-74 (como hace nv-74 con sus fuentes).
type Repository struct {
	col          *mongo.Collection
	estadisticas *mongo.Collection
	resenas      *mongo.Collection
	cupones      *mongo.Collection
	cartera      *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), estadisticas: db.Collection(estadisticasCol), resenas: db.Collection(resenasCol),
		cupones: db.Collection(cuponesCol), cartera: db.Collection(carteraCol)}
}

// Buscar trae el consejo del negocio en ese día; nil si no hay.
func (r *Repository) Buscar(ctx context.Context, negocio bson.ObjectID, dia string) (*Consejo, error) {
	var c Consejo
	err := r.col.FindOne(ctx, bson.D{{Key: "negocio_id", Value: negocio}, {Key: "dia", Value: dia}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Tomar inserta el documento en `generando`; ErrOcupado si otra petición ganó.
func (r *Repository) Tomar(ctx context.Context, c Consejo) error {
	_, err := r.col.InsertOne(ctx, c)
	if mongo.IsDuplicateKeyError(err) {
		return ErrOcupado
	}
	return err
}

// Retomar toma un documento que quedó en `generando` desde antes de `vencido` (la instancia que
// lo generaba murió). Devuelve false si otra petición lo retomó o ya está listo.
func (r *Repository) Retomar(ctx context.Context, id bson.ObjectID, vencido, ahora time.Time) (bool, error) {
	res, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: EstadoGenerando}, {Key: "creado_en", Value: bson.D{{Key: "$lt", Value: vencido}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "creado_en", Value: ahora}}}})
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

// Terminar guarda el texto y deja el documento listo.
func (r *Repository) Terminar(ctx context.Context, c Consejo) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: c.ID}}, bson.D{{Key: "$set", Value: bson.D{
		{Key: "texto", Value: c.Texto}, {Key: "accion", Value: c.Accion}, {Key: "fuente", Value: c.Fuente},
		{Key: "regla", Value: c.Regla}, {Key: "estado", Value: EstadoListo},
	}}})
	return err
}

// ConIADelDia cuenta los consejos redactados con IA ese día en toda la app (cupo IA_CONSEJO_DIA).
func (r *Repository) ConIADelDia(ctx context.Context, dia string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{{Key: "dia", Value: dia}, {Key: "fuente", Value: FuenteIA}})
}

// BorrarNegocio borra los consejos del negocio (baja, nv-56).
func (r *Repository) BorrarNegocio(ctx context.Context, negocio bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "negocio_id", Value: negocio}})
	return err
}

// Dias trae los días de `estadisticas_diarias` desde `desde` (ux_negocio_fecha), solo cifras.
func (r *Repository) Dias(ctx context.Context, negocio bson.ObjectID, desde string) ([]Dia, error) {
	cur, err := r.estadisticas.Find(ctx, bson.D{{Key: "negocio_id", Value: negocio}, {Key: "fecha", Value: bson.D{{Key: "$gte", Value: desde}}}},
		options.Find().SetProjection(bson.D{{Key: "fecha", Value: 1}, {Key: "vistas_ficha", Value: 1}, {Key: "reservas", Value: 1}, {Key: "llegadas", Value: 1},
			{Key: "cupones_canjeados", Value: 1}, {Key: "favoritos_nuevos", Value: 1}, {Key: "por_hora", Value: 1}, {Key: "resenas_dia", Value: 1}, {Key: "aspectos_suma", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var out []Dia
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResenasSinRespuesta cuenta las reseñas publicadas desde `desde` sin respuesta del negocio
// (ix_resenas_recientes). Solo el conteo: ningún texto sale de aquí.
func (r *Repository) ResenasSinRespuesta(ctx context.Context, negocio bson.ObjectID, desde time.Time) (int, error) {
	n, err := r.resenas.CountDocuments(ctx, bson.D{{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: "publicada"},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}}}, {Key: "respuesta_negocio", Value: bson.D{{Key: "$exists", Value: false}}}})
	return int(n), err
}

// CuponesSinCanjes cuenta los cupones activos creados antes de `desde` sin ningún canje desde
// entonces (ix_cupones_negocio e ix_canjes_negocio).
func (r *Repository) CuponesSinCanjes(ctx context.Context, negocio bson.ObjectID, desde time.Time) (int, error) {
	var activos []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	cur, err := r.cupones.Find(ctx, bson.D{{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: "activo"}, {Key: "creado_en", Value: bson.D{{Key: "$lte", Value: desde}}}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return 0, err
	}
	if err := cur.All(ctx, &activos); err != nil {
		return 0, err
	}
	if len(activos) == 0 {
		return 0, nil
	}
	res := r.cartera.Distinct(ctx, "cupon_id", bson.D{{Key: "negocio_id", Value: negocio}, {Key: "canjeado_en", Value: bson.D{{Key: "$gte", Value: desde}}}})
	var canjeados []bson.ObjectID
	if err := res.Decode(&canjeados); err != nil {
		return 0, err
	}
	con := make(map[bson.ObjectID]bool, len(canjeados))
	for _, id := range canjeados {
		con[id] = true
	}
	n := 0
	for _, c := range activos {
		if !con[c.ID] {
			n++
		}
	}
	return n, nil
}
