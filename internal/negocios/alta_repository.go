package negocios

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Errores del alta (nv-55).
var (
	// ErrNotFound: el negocio no existe.
	ErrNotFound = errors.New("negocios: no encontrado")
	// ErrNoAplica: el filtro condicionado (estado, dueño o versión) no coincidió.
	ErrNoAplica = errors.New("negocios: el negocio cambió o no está en ese estado")
)

// OrigenAlta marca los negocios creados por su dueño (nv-55); la semilla no lo lleva. La baja
// por ARCO solo toca estos: un administrador que borra su cuenta no se lleva la semilla.
const OrigenAlta = "alta"

// OpenStates son los estados de un alta abierta: cuentan para el tope de 3 (nv-55).
var OpenStates = []string{StatusDraft, StatusReview}

// QueuePosition es la posición del cursor de la cola del administrador.
type QueuePosition struct {
	SentAt time.Time
	ID     bson.ObjectID
}

// InsertDraft crea un negocio en borrador. No escribe `ubicacion`: el índice 2dsphere rechaza
// un punto vacío, y el borrador aún no lo tiene (paso 2).
func (r *Repository) InsertDraft(ctx context.Context, b *Business) error {
	doc := bson.D{
		{Key: "_id", Value: b.ID},
		{Key: "propietario_id", Value: b.OwnerID},
		{Key: "slug", Value: b.Slug},
		{Key: "nombre", Value: b.Name},
		{Key: "nombre_normalizado", Value: b.NormalizedName},
		{Key: "categoria", Value: b.Category},
		{Key: "etiquetas", Value: bson.A{}},
		{Key: "descripcion", Value: b.Description},
		{Key: "horarios", Value: bson.A{}},
		{Key: "horario_min", Value: bson.A{}},
		{Key: "fotos", Value: bson.A{}},
		{Key: "servicios", Value: bson.A{}},
		{Key: "opciones_dieta", Value: bson.A{}},
		{Key: "alergenos", Value: bson.A{}},
		{Key: "calificacion", Value: Calificacion{}},
		{Key: "estado", Value: StatusDraft},
		{Key: "plan", Value: PlanFree},
		{Key: "verificado", Value: false},
		{Key: "sello_excelencia", Value: false},
		{Key: "cupones_activos", Value: 0},
		{Key: "qr_checkin", Value: b.QRCheckin},
		{Key: "paso_alta", Value: 1},
		{Key: "revision", Value: Revision{}},
		{Key: "documentos", Value: Documentos{}},
		{Key: "origen", Value: OrigenAlta},
		{Key: "version", Value: 1},
		{Key: "creado_en", Value: b.CreatedAt},
		{Key: "actualizado_en", Value: b.CreatedAt},
	}
	_, err := r.col.InsertOne(ctx, doc)
	return err
}

// FindByID trae un negocio, eliminado o no.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Business, error) {
	var b Business
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Business{}, ErrNotFound
	}
	return b, err
}

// CountOpenByOwner cuenta las altas abiertas (borrador o en revisión) de la persona.
func (r *Repository) CountOpenByOwner(ctx context.Context, owner bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{
		{Key: "propietario_id", Value: owner},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: OpenStates}}},
		{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}},
	})
}

// ListByOwner trae los negocios no eliminados de la persona, del más nuevo al más viejo.
func (r *Repository) ListByOwner(ctx context.Context, owner bson.ObjectID) ([]Business, error) {
	cur, err := r.col.Find(ctx, bson.D{
		{Key: "propietario_id", Value: owner},
		{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}},
	}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}).SetLimit(50))
	if err != nil {
		return nil, err
	}
	out := []Business{}
	return out, cur.All(ctx, &out)
}

// ApplyIf aplica `$set` (más `actualizado_en` y `version + 1`) si el negocio cumple el filtro
// y devuelve el documento nuevo. ErrNoAplica si no coincide.
func (r *Repository) ApplyIf(ctx context.Context, filter, set bson.D) (Business, error) {
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	var b Business
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Business{}, ErrNoAplica
	}
	return b, err
}

// Queue trae hasta limit+1 negocios del estado, del envío más viejo al más nuevo (ix_revision).
func (r *Repository) Queue(ctx context.Context, state string, after *QueuePosition, limit int) ([]Business, error) {
	filter := bson.D{{Key: "estado", Value: state}, {Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}}}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "revision.enviado_en", Value: bson.D{{Key: "$gt", Value: after.SentAt}}}},
			bson.D{{Key: "revision.enviado_en", Value: after.SentAt}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}})
	}
	cur, err := r.col.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "revision.enviado_en", Value: 1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limit+1)))
	if err != nil {
		return nil, err
	}
	out := []Business{}
	return out, cur.All(ctx, &out)
}

// SoftDeleteByOwner da de baja los negocios de la persona (ARCO, nv-55): dejan de verse en
// `/lugares` y no vuelven a revisión. Devuelve los ids para invalidar sus fichas.
func (r *Repository) SoftDeleteByOwner(ctx context.Context, owner bson.ObjectID, at time.Time) ([]bson.ObjectID, error) {
	filter := bson.D{{Key: "propietario_id", Value: owner}, {Key: "origen", Value: OrigenAlta}, {Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}}}
	cur, err := r.col.Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if _, err := r.col.UpdateMany(ctx, filter, bson.D{
		{Key: "$set", Value: bson.D{{Key: "eliminado_en", Value: at}, {Key: "actualizado_en", Value: at}}},
		{Key: "$unset", Value: bson.D{{Key: "cuenta_deposito", Value: ""}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	}); err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

// SetDocumentos actualiza las banderas del expediente. No sube `version`: son derivadas y no
// deben invalidar el If-Match del alta.
func (r *Repository) SetDocumentos(ctx context.Context, id bson.ObjectID, d Documentos) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: bson.D{{Key: "documentos", Value: d}}}})
	return err
}

// SetCuentaDeposito guarda la cuenta enmascarada; tampoco sube `version`.
func (r *Repository) SetCuentaDeposito(ctx context.Context, id bson.ObjectID, c CuentaMascara) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: bson.D{{Key: "cuenta_deposito", Value: c}}}})
	return err
}

// InvalidarFicha borra la ficha de la caché (nv-18) tras un cambio fuera de la importación.
func (s *Service) InvalidarFicha(ctx context.Context, id string) {
	s.cache.Invalidar(context.WithoutCancel(ctx), claveFicha(id), claveCategorias)
}

// Reglas compartidas con el alta (nv-55): las mismas de la semilla.

// ValidateLocation revisa un Point GeoJSON dentro de México.
func ValidateLocation(l Location) []apperr.FieldError {
	return collect(func(add func(string, string)) { validateLocation(l, add) })
}

// ValidateAddress revisa la dirección de A2.
func ValidateAddress(d Direccion) []apperr.FieldError {
	return collect(func(add func(string, string)) { validateAddress(d, add) })
}

// ValidateSchedules revisa los horarios (al menos un día abierto).
func ValidateSchedules(h []Horario) []apperr.FieldError {
	return collect(func(add func(string, string)) { validateSchedules(h, add) })
}

// ValidatePricing revisa nivel y precio promedio.
func ValidatePricing(level, average int) []apperr.FieldError {
	return collect(func(add func(string, string)) {
		validatePricing(ImportRow{PriceLevel: level, AveragePrice: average}, add)
	})
}

// ValidateOffer revisa servicios, dietas y alérgenos con los catálogos de nv-20.
func ValidateOffer(services, diet, allergens []string) []apperr.FieldError {
	return collect(func(add func(string, string)) {
		validateList("servicios", services, MaxServices, add)
		validateCatalog("opciones_dieta", diet, MaxDiet, ValidDietOption, add)
		validateAllergens(allergens, add)
	})
}

// DeriveScheduleMin calcula `horario_min` (nv-21) desde los horarios.
func DeriveScheduleMin(h []Horario) []VentanaMin { return deriveScheduleMin(h) }

// NewQR genera un `qr_checkin` opaco.
func NewQR() (string, error) { return randomQR() }

func collect(run func(add func(string, string))) []apperr.FieldError {
	var errs []apperr.FieldError
	run(func(field, message string) { errs = append(errs, apperr.FieldError{Field: field, Message: message}) })
	return errs
}
