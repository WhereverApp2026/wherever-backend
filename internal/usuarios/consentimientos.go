package usuarios

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// ConsentsCollection es el historial de aceptaciones de términos y aviso (spec nv-19).
const ConsentsCollection = "consentimientos"

// Consent es un documento de `consentimientos`. Solo se agrega: nunca se edita. La aceptación
// vigente se copia además a `usuarios.privacidad` (nv-05) para leerla sin otra consulta.
type Consent struct {
	ID            bson.ObjectID `bson:"_id"`
	UserID        bson.ObjectID `bson:"usuario_id"`
	TermsVersion  string        `bson:"version_terminos"`
	NoticeVersion string        `bson:"version_aviso"`
	PromoEmail    bool          `bson:"promociones_correo"`
	BusinessData  bool          `bson:"datos_negocio,omitempty"`
	AcceptedAt    time.Time     `bson:"fecha_aceptacion"`
}

// ConsentRepository escribe y lee `consentimientos`.
type ConsentRepository struct {
	col *mongo.Collection
}

// NewConsentRepository recibe la base por constructor.
func NewConsentRepository(db *mongo.Database) *ConsentRepository {
	return &ConsentRepository{col: db.Collection(ConsentsCollection)}
}

// ConsentIndexes es el índice de `consentimientos`: el historial de una persona, del más nuevo al más viejo.
func ConsentIndexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ConsentsCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "fecha_aceptacion", Value: -1}}, Options: options.Index().SetName("ix_usuario_fecha")},
		},
	}}
}

// Append agrega una aceptación; fija id y fecha si faltan.
func (r *ConsentRepository) Append(ctx context.Context, c *Consent) error {
	if c.ID.IsZero() {
		c.ID = bson.NewObjectID()
	}
	if c.AcceptedAt.IsZero() {
		c.AcceptedAt = time.Now().UTC()
	}
	_, err := r.col.InsertOne(ctx, c)
	return err
}

// History devuelve las aceptaciones de la persona, de la más nueva a la más vieja.
func (r *ConsentRepository) History(ctx context.Context, userID bson.ObjectID) ([]Consent, error) {
	cur, err := r.col.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}},
		options.Find().SetSort(bson.D{{Key: "fecha_aceptacion", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, err
	}
	out := []Consent{}
	return out, cur.All(ctx, &out)
}

// ConsentPrivacy es la parte de `consentimientos` en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter.
type ConsentPrivacy struct {
	repo *ConsentRepository
}

// NewConsentPrivacy recibe el repositorio por constructor.
func NewConsentPrivacy(repo *ConsentRepository) *ConsentPrivacy { return &ConsentPrivacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *ConsentPrivacy) Name() string { return ConsentsCollection }

// PurgeHistory conserva las aceptaciones: no son historial de uso, son la prueba del consentimiento.
func (p *ConsentPrivacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra las aceptaciones con la cuenta.
func (p *ConsentPrivacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	_, err := p.repo.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// ConsentItem es una aceptación en "Descargar mis datos" y en la respuesta de la API.
type ConsentItem struct {
	TermsVersion  string    `json:"version_terminos"`
	NoticeVersion string    `json:"version_aviso_privacidad"`
	PromoEmail    bool      `json:"promociones_correo"`
	BusinessData  bool      `json:"datos_negocio,omitempty"`
	AcceptedAt    time.Time `json:"fecha_aceptacion"`
}

// Item convierte el documento en su forma pública (sin ids).
func (c Consent) Item() ConsentItem {
	return ConsentItem{TermsVersion: c.TermsVersion, NoticeVersion: c.NoticeVersion, PromoEmail: c.PromoEmail, BusinessData: c.BusinessData, AcceptedAt: c.AcceptedAt.UTC()}
}

// Export devuelve el historial de aceptaciones, de la más nueva a la más vieja.
func (p *ConsentPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.History(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ConsentItem, 0, len(rows))
	for _, c := range rows {
		out = append(out, c.Item())
	}
	return out, nil
}
