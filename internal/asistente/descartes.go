package asistente

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// DescartesCollection guarda cada parada de la IA que la verificación rechazó, para medir la
// tasa de alucinación (nv-32). Sin texto del usuario ni id de la persona: no es dato personal
// y no entra a ARCO.
const DescartesCollection = "ia_descartes"

// VidaDescarte es el TTL de un descarte.
const VidaDescarte = 30 * 24 * time.Hour

// Descartes registra los descartes.
type Descartes interface {
	Registrar(ctx context.Context, modelo string, ds []Descarte) error
}

type registroDescarte struct {
	ID        bson.ObjectID  `bson:"_id"`
	Motivo    string         `bson:"motivo"`
	NegocioID *bson.ObjectID `bson:"negocio_id,omitempty"`
	Modelo    string         `bson:"modelo"`
	CreadoEn  time.Time      `bson:"creado_en"`
	ExpiraEn  time.Time      `bson:"expira_en"`
}

// DescartesRepo escribe `ia_descartes`.
type DescartesRepo struct {
	col *mongo.Collection
	now func() time.Time
}

// NewDescartes recibe la base por constructor.
func NewDescartes(db *mongo.Database) *DescartesRepo {
	return &DescartesRepo{col: db.Collection(DescartesCollection), now: time.Now}
}

// Indexes: el TTL de 30 días y el conteo por motivo. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: DescartesCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_descarte").SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "motivo", Value: 1}, {Key: "creado_en", Value: -1}}, Options: options.Index().SetName("ix_motivo_fecha")},
		},
	}}
}

// Registrar inserta los descartes de una generación. Sin descartes no toca la base.
func (r *DescartesRepo) Registrar(ctx context.Context, modelo string, ds []Descarte) error {
	if len(ds) == 0 {
		return nil
	}
	now := r.now().UTC()
	docs := make([]any, 0, len(ds))
	for _, d := range ds {
		doc := registroDescarte{ID: bson.NewObjectID(), Motivo: d.Motivo, Modelo: modelo, CreadoEn: now, ExpiraEn: now.Add(VidaDescarte)}
		if !d.NegocioID.IsZero() {
			id := d.NegocioID
			doc.NegocioID = &id
		}
		docs = append(docs, doc)
	}
	_, err := r.col.InsertMany(ctx, docs)
	return err
}
