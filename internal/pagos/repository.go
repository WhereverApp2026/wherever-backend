package pagos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio.
var (
	ErrNotFound  = errors.New("pago no encontrado")
	ErrDuplicate = errors.New("pago repetido")
	ErrNoAplica  = errors.New("el pago cambió")
)

// Indexes son los de J5 y A5 (los crea scripts/indexes). `ttl_conservar_hasta` borra cada
// pago y liquidación al cumplir 5 años sin trabajo propio.
func Indexes() []platformmongo.IndexSet {
	ttl := func(name string) mongo.IndexModel {
		return mongo.IndexModel{Keys: bson.D{{Key: "conservar_hasta", Value: 1}}, Options: options.Index().SetName(name).SetExpireAfterSeconds(0)}
	}
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "payment_intent_id", Value: 1}},
				Options: options.Index().SetName("ux_payment_intent").SetUnique(true),
			},
			{Keys: bson.D{{Key: "idempotency_key", Value: 1}}, Options: options.Index().SetName("ux_idempotency").SetUnique(true)},
			{Keys: bson.D{{Key: "reserva_id", Value: 1}, {Key: "creado_en", Value: -1}}, Options: options.Index().SetName("ix_pago_reserva")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "fecha_reserva", Value: 1}}, Options: options.Index().SetName("ix_conciliacion")},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}}, Options: options.Index().SetName("ix_pago_usuario")},
			{
				Keys:    bson.D{{Key: "negocio_id", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("ix_pago_negocio"),
			},
			ttl("ttl_conservar_hasta"),
		}},
		{Collection: LiquidacionesCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "semana_inicio", Value: -1}}, Options: options.Index().SetName("ux_liquidacion").SetUnique(true)},
			{Keys: bson.D{{Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_liquidacion_estado")},
			ttl("ttl_liquidacion_conservar"),
		}},
	}
}

// Repository lee y escribe `pagos`, `liquidaciones` y `negocios.cobros`.
type Repository struct {
	col      *mongo.Collection
	liq      *mongo.Collection
	negocios *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), liq: db.Collection(LiquidacionesCollection), negocios: db.Collection(negocios.Collection)}
}

// Insert guarda el pago; ErrDuplicate si choca un índice único.
func (r *Repository) Insert(ctx context.Context, p *Pago) error {
	_, err := r.col.InsertOne(ctx, p)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) one(ctx context.Context, filter bson.D, opts ...options.Lister[options.FindOneOptions]) (Pago, error) {
	var p Pago
	err := r.col.FindOne(ctx, filter, opts...).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Pago{}, ErrNotFound
	}
	return p, err
}

// FindByID trae un pago.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Pago, error) {
	return r.one(ctx, bson.D{{Key: "_id", Value: id}})
}

// FindByIntent trae el pago de un PaymentIntent.
func (r *Repository) FindByIntent(ctx context.Context, intent string) (Pago, error) {
	return r.one(ctx, bson.D{{Key: "payment_intent_id", Value: intent}})
}

// UltimoDeReserva trae el pago más nuevo de la reserva.
func (r *Repository) UltimoDeReserva(ctx context.Context, reserva bson.ObjectID) (Pago, error) {
	return r.one(ctx, bson.D{{Key: "reserva_id", Value: reserva}}, options.FindOne().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
}

// Intentos cuenta los pagos de la reserva (el siguiente intento es n + 1).
func (r *Repository) Intentos(ctx context.Context, reserva bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{{Key: "reserva_id", Value: reserva}})
}

// Aplicar cambia el pago si está en uno de `estados` y no procesó ya el evento (respaldo
// durable de la idempotencia: guarda los últimos 20 ids). `evento` vacío = sin registro.
func (r *Repository) Aplicar(ctx context.Context, id bson.ObjectID, estados []string, evento string, set bson.D, push bson.D) (Pago, error) {
	filter := bson.D{{Key: "_id", Value: id}}
	if len(estados) > 0 {
		filter = append(filter, bson.E{Key: "estado", Value: bson.D{{Key: "$in", Value: estados}}})
	}
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	update := bson.D{{Key: "$set", Value: set}}
	if evento != "" {
		filter = append(filter, bson.E{Key: "eventos_procesados", Value: bson.D{{Key: "$ne", Value: evento}}})
		push = append(push, bson.E{Key: "eventos_procesados", Value: bson.D{{Key: "$each", Value: bson.A{evento}}, {Key: "$slice", Value: -MaxEventos}}})
	}
	if len(push) > 0 {
		update = append(update, bson.E{Key: "$push", Value: push})
	}
	var p Pago
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Pago{}, ErrNoAplica
	}
	return p, err
}

// DeUsuario son los pagos de la persona (exportación ARCO).
func (r *Repository) DeUsuario(ctx context.Context, usuario bson.ObjectID) ([]Pago, error) {
	return findAll[Pago](ctx, r.col, bson.D{{Key: "usuario_id", Value: usuario}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
}

// PorLiquidar son los pagos cobrados de reservas en [desde, hasta) que no entraron a otra
// liquidación.
func (r *Repository) PorLiquidar(ctx context.Context, desde, hasta time.Time, negocio *bson.ObjectID) ([]Pago, error) {
	filter := bson.D{
		{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{EstadoAprobado, EstadoReembolsadoParcial}}}},
		{Key: "fecha_reserva", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}},
		{Key: "liquidacion_semana", Value: bson.D{{Key: "$exists", Value: false}}},
	}
	if negocio != nil {
		filter = append(filter, bson.E{Key: "negocio_id", Value: *negocio})
	}
	return findAll[Pago](ctx, r.col, filter, options.Find())
}

// FiltroCobros es la consulta de la lista de cobros: el negocio, el rango [Desde, Hasta) de
// `creado_en` (nil = abierto) y el estado ("" = todos).
type FiltroCobros struct {
	Negocio bson.ObjectID
	Desde   *time.Time
	Hasta   *time.Time
	Estado  string
}

// Posicion es el último cobro entregado (cursor).
type Posicion struct {
	CreadoEn time.Time
	ID       bson.ObjectID
}

// filtro arma la consulta con `estado`: un valor, un operador o "" (sin filtro de estado).
func (f FiltroCobros) filtro(estado any) bson.D {
	filter := bson.D{{Key: "negocio_id", Value: f.Negocio}}
	rango := bson.D{}
	if f.Desde != nil {
		rango = append(rango, bson.E{Key: "$gte", Value: *f.Desde})
	}
	if f.Hasta != nil {
		rango = append(rango, bson.E{Key: "$lt", Value: *f.Hasta})
	}
	if len(rango) > 0 {
		filter = append(filter, bson.E{Key: "creado_en", Value: rango})
	}
	if e, ok := estado.(string); !ok || e != "" {
		filter = append(filter, bson.E{Key: "estado", Value: estado})
	}
	return filter
}

// Cobros son los pagos del filtro del más nuevo al más viejo, después de `despues` (cursor).
func (r *Repository) Cobros(ctx context.Context, f FiltroCobros, despues *Posicion, limit int) ([]Pago, error) {
	filter := f.filtro(f.Estado)
	if despues != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$lt", Value: despues.CreadoEn}}}},
			bson.D{{Key: "creado_en", Value: despues.CreadoEn}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: despues.ID}}}},
		}})
	}
	sort := bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}
	return findAll[Pago](ctx, r.col, filter, options.Find().SetSort(sort).SetLimit(int64(limit+1)))
}

// CobradosDe son los pagos cobrados del filtro (solo montos), para los totales del rango.
func (r *Repository) CobradosDe(ctx context.Context, f FiltroCobros) ([]Pago, error) {
	estados := bson.A{}
	for _, e := range Cobrados {
		if f.Estado == "" || f.Estado == e {
			estados = append(estados, e)
		}
	}
	if len(estados) == 0 {
		return nil, nil
	}
	proj := bson.D{{Key: "monto_centavos", Value: 1}, {Key: "comision_centavos", Value: 1}, {Key: "reembolsos.monto_centavos", Value: 1}}
	return findAll[Pago](ctx, r.col, f.filtro(bson.D{{Key: "$in", Value: estados}}), options.Find().SetProjection(proj))
}

// MarcarLiquidados deja la semana en los pagos del corte.
func (r *Repository) MarcarLiquidados(ctx context.Context, ids []bson.ObjectID, semana time.Time) error {
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "liquidacion_semana", Value: semana}, {Key: "actualizado_en", Value: time.Now().UTC()}}}})
	return err
}

// InsertLiquidacion guarda la liquidación; ErrDuplicate si la semana ya existe.
func (r *Repository) InsertLiquidacion(ctx context.Context, l *Liquidacion) error {
	_, err := r.liq.InsertOne(ctx, l)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

// LiquidacionesPendientes son las que esperan transferencia.
func (r *Repository) LiquidacionesPendientes(ctx context.Context, negocio *bson.ObjectID) ([]Liquidacion, error) {
	filter := bson.D{{Key: "estado", Value: LiquidacionPendiente}}
	if negocio != nil {
		filter = append(filter, bson.E{Key: "negocio_id", Value: *negocio})
	}
	return findAll[Liquidacion](ctx, r.liq, filter, options.Find().SetSort(bson.D{{Key: "semana_inicio", Value: 1}}))
}

// Transferida marca la liquidación con su transferencia.
func (r *Repository) Transferida(ctx context.Context, id bson.ObjectID, transfer string) error {
	_, err := r.liq.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: LiquidacionPendiente}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: LiquidacionTransferida}, {Key: "stripe_transfer_id", Value: transfer}, {Key: "actualizado_en", Value: time.Now().UTC()}}}})
	return err
}

// Liquidaciones del negocio de la más nueva a la más vieja, después de `antes` (cursor).
func (r *Repository) Liquidaciones(ctx context.Context, negocio bson.ObjectID, antes *time.Time, limit int) ([]Liquidacion, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}}
	if antes != nil {
		filter = append(filter, bson.E{Key: "semana_inicio", Value: bson.D{{Key: "$lt", Value: *antes}}})
	}
	return findAll[Liquidacion](ctx, r.liq, filter, options.Find().SetSort(bson.D{{Key: "semana_inicio", Value: -1}}).SetLimit(int64(limit+1)))
}

// LiquidacionesDeNegocios son todas las de esos negocios (exportación del comerciante).
func (r *Repository) LiquidacionesDeNegocios(ctx context.Context, ids []bson.ObjectID) ([]Liquidacion, error) {
	return findAll[Liquidacion](ctx, r.liq, bson.D{{Key: "negocio_id", Value: bson.D{{Key: "$in", Value: ids}}}}, options.Find().SetSort(bson.D{{Key: "semana_inicio", Value: -1}}))
}

// NegociosDe son los ids de los negocios del dueño.
func (r *Repository) NegociosDe(ctx context.Context, owner bson.ObjectID) ([]bson.ObjectID, error) {
	rows, err := findAll[struct {
		ID bson.ObjectID `bson:"_id"`
	}](ctx, r.negocios, bson.D{{Key: "propietario_id", Value: owner}}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out, nil
}

// SetCobros guarda la cuenta de Connect del negocio.
func (r *Repository) SetCobros(ctx context.Context, negocio bson.ObjectID, c negocios.Cobros) error {
	_, err := r.negocios.UpdateOne(ctx, bson.D{{Key: "_id", Value: negocio}}, bson.D{{Key: "$set", Value: bson.D{{Key: "cobros", Value: c}}}})
	return err
}

// EstadoCobros cambia `cobros.estado` de la cuenta de Stripe (`account.updated`).
func (r *Repository) EstadoCobros(ctx context.Context, account, estado string) error {
	_, err := r.negocios.UpdateOne(ctx, bson.D{{Key: "cobros.stripe_account_id", Value: account}}, bson.D{{Key: "$set", Value: bson.D{{Key: "cobros.estado", Value: estado}}}})
	return err
}

// CuentaRegistrada marca la cuenta de depósito como registrada en Stripe.
func (r *Repository) CuentaRegistrada(ctx context.Context, negocio bson.ObjectID, externa string) error {
	_, err := r.negocios.UpdateOne(ctx, bson.D{{Key: "_id", Value: negocio}, {Key: "cuenta_deposito", Value: bson.D{{Key: "$exists", Value: true}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "cuenta_deposito.estado", Value: "registrada"}, {Key: "cuenta_deposito.stripe_external_account_id", Value: externa}}}})
	return err
}

func findAll[T any](ctx context.Context, col *mongo.Collection, filter bson.D, opts *options.FindOptionsBuilder) ([]T, error) {
	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var out []T
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
