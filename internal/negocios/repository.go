package negocios

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Indexes son los 9 índices de A2 §1 y A5 §1. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	exists := func(field string) bson.D {
		return bson.D{{Key: field, Value: bson.D{{Key: "$exists", Value: true}}}}
	}
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{
				{Key: "ubicacion", Value: "2dsphere"},
				{Key: "estado", Value: 1},
				{Key: "categoria", Value: 1},
				{Key: "nivel_precio", Value: 1},
			}, Options: options.Index().SetName("ix_geo_filtros")},
			{Keys: bson.D{
				{Key: "nombre", Value: "text"},
				{Key: "etiquetas", Value: "text"},
				{Key: "descripcion", Value: "text"},
			}, Options: options.Index().
				SetName("ix_texto").
				SetDefaultLanguage("spanish").
				SetWeights(bson.D{
					{Key: "nombre", Value: 10},
					{Key: "etiquetas", Value: 5},
					{Key: "descripcion", Value: 1},
				})},
			{Keys: bson.D{
				{Key: "estado", Value: 1},
				{Key: "nombre_normalizado", Value: 1},
			}, Options: options.Index().SetName("ix_autocompletar")},
			{Keys: bson.D{
				{Key: "estado", Value: 1},
				{Key: "categoria", Value: 1},
				{Key: "calificacion.promedio", Value: -1},
			}, Options: options.Index().SetName("ix_categoria_calif")},
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetName("ux_slug").SetUnique(true)},
			{Keys: bson.D{{Key: "propietario_id", Value: 1}}, Options: options.Index().SetName("ix_propietario")},
			{Keys: bson.D{{Key: "destacado_hasta", Value: -1}}, Options: options.Index().SetName("ix_destacados").SetPartialFilterExpression(exists("destacado_hasta"))},
			{Keys: bson.D{{Key: "qr_checkin", Value: 1}}, Options: options.Index().SetName("ux_qr_checkin").SetUnique(true)},
			{Keys: bson.D{
				{Key: "estado", Value: 1},
				{Key: "revision.enviado_en", Value: 1},
			}, Options: options.Index().SetName("ix_revision")},
		},
	}}
}

// Repository accede a `negocios`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// VisibleFilter es el filtro compartido: solo aprobados y no eliminados.
func VisibleFilter() bson.D {
	return bson.D{
		{Key: "estado", Value: StatusApproved},
		{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}},
	}
}

// Upsert crea o actualiza un negocio por `slug`. Devuelve true si lo insertó.
// `$set` actualiza el catálogo y la revisión; `$setOnInsert` fija los campos que una
// reimportación no debe reiniciar (calificación, cupones, sello, plan y QR).
func (r *Repository) Upsert(ctx context.Context, b *Business) (bool, error) {
	now := time.Now().UTC()
	if b.ID.IsZero() {
		b.ID = bson.NewObjectID()
	}
	if b.NormalizedName == "" {
		b.NormalizedName = text.Normalize(b.Name)
	}
	set := bson.D{
		{Key: "propietario_id", Value: b.OwnerID},
		{Key: "slug", Value: b.Slug},
		{Key: "nombre", Value: b.Name},
		{Key: "nombre_normalizado", Value: b.NormalizedName},
		{Key: "categoria", Value: b.Category},
		{Key: "etiquetas", Value: b.Tags},
		{Key: "descripcion", Value: b.Description},
		{Key: "nivel_precio", Value: b.PriceLevel},
		{Key: "precio_promedio", Value: b.AveragePrice},
		{Key: "ubicacion", Value: b.Location},
		{Key: "direccion", Value: b.Address},
		{Key: "telefono", Value: b.Phone},
		{Key: "horarios", Value: b.Schedules},
		{Key: "horario_min", Value: b.ScheduleMin},
		{Key: "servicios", Value: b.Services},
		{Key: "accesibilidad", Value: b.Accessibility},
		{Key: "opciones_dieta", Value: b.DietOptions},
		{Key: "alergenos", Value: b.Allergens},
		{Key: "acepta_reservas", Value: b.AcceptsReservations},
		{Key: "estado", Value: StatusApproved},
		{Key: "paso_alta", Value: StepComplete},
		{Key: "revision", Value: b.Revision},
		{Key: "actualizado_en", Value: now},
	}
	// Una fila sin fotos no borra las que el negocio ya tenga (subidas después por el panel).
	if len(b.Photos) > 0 {
		set = append(set, bson.E{Key: "fotos", Value: b.Photos})
	}
	setOnInsert := bson.D{
		{Key: "_id", Value: b.ID},
		{Key: "qr_checkin", Value: b.QRCheckin},
		{Key: "creado_en", Value: now},
		{Key: "calificacion", Value: b.Rating},
		{Key: "cupones_activos", Value: 0},
		{Key: "sello_excelencia", Value: false},
		{Key: "verificado", Value: false},
		{Key: "plan", Value: PlanFree},
	}
	if len(b.Photos) == 0 {
		setOnInsert = append(setOnInsert, bson.E{Key: "fotos", Value: []Foto{}})
	}
	update := bson.D{
		{Key: "$set", Value: set},
		{Key: "$setOnInsert", Value: setOnInsert},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	}
	// El documento anterior da el _id real al actualizar, para invalidar su ficha (nv-18).
	var antes struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "slug", Value: b.Slug}}, update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.Before).
			SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&antes)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	b.ID = antes.ID
	return false, nil
}

// FacetCount es un conteo agrupado por un valor.
type FacetCount struct {
	ID    string `bson:"_id"`
	Count int    `bson:"conteo"`
}

// FacetCounts son los conteos crudos de la agregación; el servicio los ordena y completa
// los 6 giros con cero.
type FacetCounts struct {
	Categories []FacetCount
	Tags       []FacetCount
	Services   []FacetCount
}

// Facets calcula categorías, etiquetas y servicios sobre los negocios visibles.
func (r *Repository) Facets(ctx context.Context) (FacetCounts, error) {
	group := bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: "$categoria"},
		{Key: "conteo", Value: bson.D{{Key: "$sum", Value: 1}}},
	}}}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: VisibleFilter()}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "categorias", Value: bson.A{group}},
			{Key: "etiquetas", Value: bson.A{
				bson.D{{Key: "$unwind", Value: "$etiquetas"}},
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$etiquetas"},
					{Key: "conteo", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "conteo", Value: -1}, {Key: "_id", Value: 1}}}},
				bson.D{{Key: "$limit", Value: MaxFacets}},
			}},
			{Key: "servicios", Value: bson.A{
				bson.D{{Key: "$unwind", Value: "$servicios"}},
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$servicios"},
					{Key: "conteo", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "conteo", Value: -1}, {Key: "_id", Value: 1}}}},
				bson.D{{Key: "$limit", Value: MaxFacets}},
			}},
		}}},
	}
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return FacetCounts{}, err
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []struct {
		Categories []FacetCount `bson:"categorias"`
		Tags       []FacetCount `bson:"etiquetas"`
		Services   []FacetCount `bson:"servicios"`
	}
	if err := cur.All(ctx, &out); err != nil {
		return FacetCounts{}, err
	}
	if len(out) == 0 {
		return FacetCounts{}, nil
	}
	return FacetCounts{Categories: out[0].Categories, Tags: out[0].Tags, Services: out[0].Services}, nil
}
