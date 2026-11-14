package cupones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio.
var (
	ErrNotFound  = errors.New("cupón no encontrado")
	ErrDuplicate = errors.New("duplicado")
	ErrNoAplica  = errors.New("el documento cambió")
)

// Indexes son los índices de J6 (los crea scripts/indexes).
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "ubicacion", Value: "2dsphere"}, {Key: "estado", Value: 1}, {Key: "vigencia_fin", Value: 1}}, Options: options.Index().SetName("ix_cupones_cerca")},
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "creado_en", Value: -1}}, Options: options.Index().SetName("ix_cupones_negocio")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "vigencia_fin", Value: 1}}, Options: options.Index().SetName("ix_vencimiento")},
		}},
		{Collection: CarteraCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "cupon_id", Value: 1}}, Options: options.Index().SetName("ux_cartera").SetUnique(true)},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "vence_en", Value: 1}}, Options: options.Index().SetName("ix_cartera_filtros")},
			{
				Keys:    bson.D{{Key: "codigo_canje", Value: 1}},
				Options: options.Index().SetName("ux_codigo_canje").SetUnique(true).SetPartialFilterExpression(bson.D{{Key: "estado", Value: CarteraEnCanje}}),
			},
			{
				Keys:    bson.D{{Key: "codigo_canje_usado", Value: 1}},
				Options: options.Index().SetName("ix_codigo_usado").SetPartialFilterExpression(bson.D{{Key: "codigo_canje_usado", Value: bson.D{{Key: "$exists", Value: true}}}}),
			},
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "canjeado_en", Value: 1}}, Options: options.Index().SetName("ix_canjes_negocio")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "vence_en", Value: 1}}, Options: options.Index().SetName("ix_cartera_vence")},
		}},
		{Collection: MetricasCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "cupon_id", Value: 1}, {Key: "dia", Value: 1}}, Options: options.Index().SetName("ux_cupon_dia").SetUnique(true)},
		}},
	}
}

// Repository lee y escribe `cupones`, `cupones_usuario` y `cupones_metricas`.
type Repository struct {
	col      *mongo.Collection
	cartera  *mongo.Collection
	metricas *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), cartera: db.Collection(CarteraCollection), metricas: db.Collection(MetricasCollection)}
}

// vigenteFilter: estado usable y dentro de la vigencia (no depende del trabajo).
func vigenteFilter(now time.Time) bson.D {
	return bson.D{
		{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{EstadoActivo, EstadoProgramado}}}},
		{Key: "vigencia_inicio", Value: bson.D{{Key: "$lte", Value: now}}},
		{Key: "vigencia_fin", Value: bson.D{{Key: "$gt", Value: now}}},
	}
}

func findAll[T any](ctx context.Context, col *mongo.Collection, filter any, opts *options.FindOptionsBuilder) ([]T, error) {
	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []T{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func unsetDe(campos []string) bson.D {
	u := bson.D{}
	for _, f := range campos {
		u = append(u, bson.E{Key: f, Value: ""})
	}
	return u
}

// --- cupones ---

// Insert guarda el cupón.
func (r *Repository) Insert(ctx context.Context, c *Cupon) error {
	_, err := r.col.InsertOne(ctx, c)
	return err
}

// FindByID trae un cupón; ErrNotFound si no existe.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Cupon, error) {
	var c Cupon
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Cupon{}, ErrNotFound
	}
	return c, err
}

// PosLista es la posición del cursor de la lista del negocio (creado_en desc, _id desc).
type PosLista struct {
	Fecha time.Time
	ID    bson.ObjectID
}

// DelNegocio lista los cupones del negocio, del más nuevo al más viejo; trae limit + 1.
func (r *Repository) DelNegocio(ctx context.Context, negocio bson.ObjectID, estado string, after *PosLista, limit int) ([]Cupon, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}}
	if estado != "" {
		filter = append(filter, bson.E{Key: "estado", Value: estado})
	}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$lt", Value: after.Fecha}}}},
			bson.D{{Key: "creado_en", Value: after.Fecha}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit + 1))
	return findAll[Cupon](ctx, r.col, filter, opts)
}

// CambiarSi aplica `set`/`unset` si el cupón es del negocio, tiene la versión y cumple `extra`;
// sube la versión. ErrNoAplica si no coincide.
func (r *Repository) CambiarSi(ctx context.Context, id, negocio bson.ObjectID, version int, extra, set bson.D, unset []string, now time.Time) (Cupon, error) {
	filter := append(bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}, {Key: "version", Value: version}}, extra...)
	update := bson.D{
		{Key: "$set", Value: append(set, bson.E{Key: "actualizado_en", Value: now})},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	}
	if len(unset) > 0 {
		update = append(update, bson.E{Key: "$unset", Value: unsetDe(unset)})
	}
	var c Cupon
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Cupon{}, ErrNoAplica
	}
	return c, err
}

// BorrarSi borra el cupón del negocio con esa versión y sin canjes. ErrNoAplica si no coincide.
func (r *Repository) BorrarSi(ctx context.Context, id, negocio bson.ObjectID, version int) error {
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}, {Key: "version", Value: version}, {Key: "usos_actuales", Value: 0}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNoAplica
	}
	return nil
}

// FilaCerca es un cupón con la distancia que calcula `$geoNear`.
type FilaCerca struct {
	Cupon      `bson:",inline"`
	DistanciaM float64 `bson:"distancia_m"`
}

// PosCerca es la posición del cursor de `cerca` (distancia y _id).
type PosCerca struct {
	Distancia float64
	ID        bson.ObjectID
}

// Cerca trae los cupones vigentes en el radio, del más cercano al más lejano; trae limit + 1.
// Por distancia la página siguiente arranca en `minDistance` y el `$match` quita los empates
// ya entregados.
func (r *Repository) Cerca(ctx context.Context, lng, lat, radioM float64, categoria string, now time.Time, after *PosCerca, limit int) ([]FilaCerca, error) {
	query := vigenteFilter(now)
	if categoria != "" {
		query = append(query, bson.E{Key: "categoria", Value: categoria})
	}
	near := bson.D{
		{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{lng, lat}}}},
		{Key: "distanceField", Value: "distancia_m"},
		{Key: "maxDistance", Value: radioM},
		{Key: "spherical", Value: true},
		{Key: "key", Value: "ubicacion"},
		{Key: "query", Value: query},
	}
	pipeline := mongo.Pipeline{}
	if after != nil {
		near = append(near, bson.E{Key: "minDistance", Value: after.Distancia})
		pipeline = append(pipeline, bson.D{{Key: "$geoNear", Value: near}}, bson.D{{Key: "$match", Value: bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "distancia_m", Value: bson.D{{Key: "$gt", Value: after.Distancia}}}},
			bson.D{{Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}}}}})
	} else {
		pipeline = append(pipeline, bson.D{{Key: "$geoNear", Value: near}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "distancia_m", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit + 1}},
	)
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []FilaCerca{}
	return out, cur.All(ctx, &out)
}

// Sumar incrementa un contador del cupón (`vistas` o `guardados`).
func (r *Repository) Sumar(ctx context.Context, id bson.ObjectID, campo string) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$inc", Value: bson.D{{Key: campo, Value: 1}}}})
	return err
}

// TomarUso suma un uso si el cupón sigue vigente y no llegó a `limite_usos` (0 = sin límite);
// al llegar pasa a `agotado`. Todo en una sola actualización atómica. ErrNoAplica si no cabe.
func (r *Repository) TomarUso(ctx context.Context, id bson.ObjectID, now time.Time) error {
	filter := append(bson.D{{Key: "_id", Value: id}}, vigenteFilter(now)...)
	filter = append(filter, bson.E{Key: "$expr", Value: bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "$lte", Value: bson.A{"$limite_usos", 0}}},
		bson.D{{Key: "$lt", Value: bson.A{"$usos_actuales", "$limite_usos"}}},
	}}}})
	update := mongo.Pipeline{
		{{Key: "$set", Value: bson.D{{Key: "usos_actuales", Value: bson.D{{Key: "$add", Value: bson.A{"$usos_actuales", 1}}}}, {Key: "actualizado_en", Value: now}}}},
		{{Key: "$set", Value: bson.D{{Key: "estado", Value: bson.D{{Key: "$cond", Value: bson.A{
			bson.D{{Key: "$and", Value: bson.A{
				bson.D{{Key: "$gt", Value: bson.A{"$limite_usos", 0}}},
				bson.D{{Key: "$gte", Value: bson.A{"$usos_actuales", "$limite_usos"}}},
			}}},
			EstadoAgotado, "$estado",
		}}}}}}},
	}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNoAplica
	}
	return nil
}

// Agotado dice si el cupón ya no tiene usos (para el motivo del rojo).
func (c Cupon) Agotado() bool {
	return c.Estado == EstadoAgotado || (c.LimiteUsos > 0 && c.UsosActuales >= c.LimiteUsos)
}

// Activar pasa a `activo` los programados cuyo inicio llegó.
func (r *Repository) Activar(ctx context.Context, now time.Time) error {
	_, err := r.col.UpdateMany(ctx, bson.D{
		{Key: "estado", Value: EstadoProgramado}, {Key: "vigencia_inicio", Value: bson.D{{Key: "$lte", Value: now}}},
	}, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoActivo}, {Key: "actualizado_en", Value: now}}}})
	return err
}

// Vencer pasa a `vencido` los activos o programados terminados y devuelve sus negocios.
func (r *Repository) Vencer(ctx context.Context, now time.Time) ([]bson.ObjectID, error) {
	filter := bson.D{
		{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{EstadoActivo, EstadoProgramado}}}},
		{Key: "vigencia_fin", Value: bson.D{{Key: "$lte", Value: now}}},
	}
	var negocios []bson.ObjectID
	if err := r.col.Distinct(ctx, "negocio_id", filter).Decode(&negocios); err != nil {
		return nil, err
	}
	if len(negocios) == 0 {
		return nil, nil
	}
	_, err := r.col.UpdateMany(ctx, filter, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoVencido}, {Key: "actualizado_en", Value: now}}}})
	return negocios, err
}

// NegociosConInicio son los negocios con cupones programados cuyo inicio llegó.
func (r *Repository) NegociosConInicio(ctx context.Context, now time.Time) ([]bson.ObjectID, error) {
	var out []bson.ObjectID
	err := r.col.Distinct(ctx, "negocio_id", bson.D{
		{Key: "estado", Value: EstadoProgramado}, {Key: "vigencia_inicio", Value: bson.D{{Key: "$lte", Value: now}}},
	}).Decode(&out)
	return out, err
}

// ContarActivos cuenta los cupones vigentes del negocio.
func (r *Repository) ContarActivos(ctx context.Context, negocio bson.ObjectID, now time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, append(bson.D{{Key: "negocio_id", Value: negocio}}, vigenteFilter(now)...))
}

// PausarDelNegocio pausa los activos o programados del negocio; `porNegocio` marca los que
// vuelven al reanudar.
func (r *Repository) PausarDelNegocio(ctx context.Context, negocio bson.ObjectID, porNegocio bool, now time.Time) error {
	set := bson.D{{Key: "estado", Value: EstadoPausado}, {Key: "actualizado_en", Value: now}}
	if porNegocio {
		set = append(set, bson.E{Key: "pausado_por_negocio", Value: true})
	}
	_, err := r.col.UpdateMany(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{EstadoActivo, EstadoProgramado}}}},
	}, bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	return err
}

// ReanudarDelNegocio devuelve a `activo` los que pausó el negocio (las lecturas filtran fechas).
func (r *Repository) ReanudarDelNegocio(ctx context.Context, negocio bson.ObjectID, now time.Time) error {
	_, err := r.col.UpdateMany(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: EstadoPausado}, {Key: "pausado_por_negocio", Value: true},
	}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoActivo}, {Key: "actualizado_en", Value: now}}},
		{Key: "$unset", Value: bson.D{{Key: "pausado_por_negocio", Value: ""}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	})
	return err
}

// --- métricas ---

// SumarMetrica incrementa `campo` del cupón en el día UTC de `now` (upsert).
func (r *Repository) SumarMetrica(ctx context.Context, cupon bson.ObjectID, campo string, now time.Time) error {
	_, err := r.metricas.UpdateOne(ctx, bson.D{{Key: "cupon_id", Value: cupon}, {Key: "dia", Value: Dia(now)}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: campo, Value: 1}}}}, options.UpdateOne().SetUpsert(true))
	return err
}

// Metricas trae los días del cupón entre desde y hasta (incluidos, `AAAA-MM-DD`).
func (r *Repository) Metricas(ctx context.Context, cupon bson.ObjectID, desde, hasta string) ([]Metrica, error) {
	return findAll[Metrica](ctx, r.metricas, bson.D{
		{Key: "cupon_id", Value: cupon}, {Key: "dia", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lte", Value: hasta}}},
	}, options.Find().SetSort(bson.D{{Key: "dia", Value: 1}}))
}

// --- cartera ---

// InsertCartera guarda el cupón en la cartera; ErrDuplicate si ya estaba (ux_cartera).
func (r *Repository) InsertCartera(ctx context.Context, c *Cartera) error {
	_, err := r.cartera.InsertOne(ctx, c)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) unaCartera(ctx context.Context, filter bson.D, opts ...options.Lister[options.FindOneOptions]) (Cartera, error) {
	var c Cartera
	err := r.cartera.FindOne(ctx, filter, opts...).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Cartera{}, ErrNotFound
	}
	return c, err
}

// CarteraDe trae un cupón guardado de la persona.
func (r *Repository) CarteraDe(ctx context.Context, usuario, id bson.ObjectID) (Cartera, error) {
	return r.unaCartera(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: usuario}})
}

// PosCartera es la posición del cursor de la cartera (fecha del orden y _id).
type PosCartera struct {
	Fecha time.Time
	ID    bson.ObjectID
}

// FiltroCartera dice qué estados y fechas entran y por qué campo se ordena.
type FiltroCartera struct {
	Estados []string
	Hasta   *time.Time // vence_en < Hasta (por vencer)
	Desde   *time.Time // vence_en > Desde (vigentes)
	Campo   string     // vence_en (asc) o canjeado_en (desc)
	Desc    bool
}

// Mias lista la cartera de la persona con el filtro; trae limit + 1.
func (r *Repository) Mias(ctx context.Context, usuario bson.ObjectID, f FiltroCartera, after *PosCartera, limit int) ([]Cartera, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}, {Key: "estado", Value: bson.D{{Key: "$in", Value: f.Estados}}}}
	rango := bson.D{}
	if f.Desde != nil {
		rango = append(rango, bson.E{Key: "$gt", Value: *f.Desde})
	}
	if f.Hasta != nil {
		rango = append(rango, bson.E{Key: "$lt", Value: *f.Hasta})
	}
	if len(rango) > 0 {
		filter = append(filter, bson.E{Key: "vence_en", Value: rango})
	}
	dir, op := 1, "$gt"
	if f.Desc {
		dir, op = -1, "$lt"
	}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: f.Campo, Value: bson.D{{Key: op, Value: after.Fecha}}}},
			bson.D{{Key: f.Campo, Value: after.Fecha}, {Key: "_id", Value: bson.D{{Key: op, Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: f.Campo, Value: dir}, {Key: "_id", Value: dir}}).SetLimit(int64(limit + 1))
	return findAll[Cartera](ctx, r.cartera, filter, opts)
}

// QuitarCartera borra un cupón guardado de la persona; ErrNotFound si no era suyo.
func (r *Repository) QuitarCartera(ctx context.Context, usuario, id bson.ObjectID) error {
	res, err := r.cartera.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: usuario}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// PonerCodigo deja la cartera `en_canje` con un código nuevo si está guardada o su código ya
// expiró. ErrNoAplica si no cumple; ErrDuplicate si el código choca (ux_codigo_canje).
func (r *Repository) PonerCodigo(ctx context.Context, usuario, id bson.ObjectID, codigo string, expira, now time.Time) (Cartera, error) {
	filter := bson.D{
		{Key: "_id", Value: id}, {Key: "usuario_id", Value: usuario},
		{Key: "vence_en", Value: bson.D{{Key: "$gt", Value: now}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "estado", Value: CarteraGuardado}},
			bson.D{{Key: "estado", Value: CarteraEnCanje}, {Key: "canje_expira_en", Value: bson.D{{Key: "$lte", Value: now}}}},
		}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "estado", Value: CarteraEnCanje}, {Key: "codigo_canje", Value: codigo}, {Key: "canje_expira_en", Value: expira}, {Key: "actualizado_en", Value: now},
	}}}
	var c Cartera
	err := r.cartera.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return Cartera{}, ErrNoAplica
	case mongo.IsDuplicateKeyError(err):
		return Cartera{}, ErrDuplicate
	}
	return c, err
}

// PorCodigo trae la cartera que tiene el código vigente o expirado sin regenerar.
func (r *Repository) PorCodigo(ctx context.Context, codigo string) (Cartera, error) {
	return r.unaCartera(ctx, bson.D{{Key: "codigo_canje", Value: codigo}, {Key: "estado", Value: CarteraEnCanje}})
}

// PorCodigoUsado trae el canje más reciente con ese código (ventana del reintento).
func (r *Repository) PorCodigoUsado(ctx context.Context, codigo string) (Cartera, error) {
	return r.unaCartera(ctx, bson.D{{Key: "codigo_canje_usado", Value: codigo}}, options.FindOne().SetSort(bson.D{{Key: "canjeado_en", Value: -1}}))
}

// Canjear marca la cartera `canjeado` si el código sigue vivo y es del negocio (atómico: de
// varios escaneos simultáneos solo uno lo encuentra). ErrNoAplica si no.
func (r *Repository) Canjear(ctx context.Context, negocio bson.ObjectID, codigo string, por bson.ObjectID, now time.Time) (Cartera, error) {
	filter := bson.D{
		{Key: "codigo_canje", Value: codigo}, {Key: "estado", Value: CarteraEnCanje},
		{Key: "canje_expira_en", Value: bson.D{{Key: "$gt", Value: now}}}, {Key: "negocio_id", Value: negocio},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "estado", Value: CarteraCanjeado}, {Key: "canjeado_en", Value: now}, {Key: "validado_por", Value: por},
			{Key: "codigo_canje_usado", Value: codigo}, {Key: "actualizado_en", Value: now},
		}},
		{Key: "$unset", Value: bson.D{{Key: "codigo_canje", Value: ""}}},
	}
	var c Cartera
	err := r.cartera.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Cartera{}, ErrNoAplica
	}
	return c, err
}

// DeshacerCanje devuelve la cartera a `en_canje` cuando el cupón ya no tenía usos.
func (r *Repository) DeshacerCanje(ctx context.Context, c Cartera, now time.Time) error {
	_, err := r.cartera.UpdateOne(ctx, bson.D{{Key: "_id", Value: c.ID}, {Key: "estado", Value: CarteraCanjeado}}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: CarteraEnCanje}, {Key: "codigo_canje", Value: c.CodigoCanjeUsado}, {Key: "actualizado_en", Value: now}}},
		{Key: "$unset", Value: bson.D{{Key: "canjeado_en", Value: ""}, {Key: "validado_por", Value: ""}, {Key: "codigo_canje_usado", Value: ""}}},
	})
	return err
}

// VencerCarteras pasa a `vencido` las guardadas o en canje cuyo cupón terminó y olvida los
// códigos usados de más de 5 minutos (el reintento ya no aplica).
func (r *Repository) VencerCarteras(ctx context.Context, now time.Time) error {
	if _, err := r.cartera.UpdateMany(ctx, bson.D{
		{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{CarteraGuardado, CarteraEnCanje}}}},
		{Key: "vence_en", Value: bson.D{{Key: "$lte", Value: now}}},
	}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: CarteraVencido}, {Key: "actualizado_en", Value: now}}},
		{Key: "$unset", Value: bson.D{{Key: "codigo_canje", Value: ""}, {Key: "canje_expira_en", Value: ""}}},
	}); err != nil {
		return err
	}
	_, err := r.cartera.UpdateMany(ctx, bson.D{
		{Key: "codigo_canje_usado", Value: bson.D{{Key: "$exists", Value: true}}},
		{Key: "canjeado_en", Value: bson.D{{Key: "$lte", Value: now.Add(-VidaCodigo)}}},
	}, bson.D{{Key: "$unset", Value: bson.D{{Key: "codigo_canje_usado", Value: ""}}}})
	return err
}

// CanjeadosEntre cuenta los canjes del negocio en [desde, hasta).
func (r *Repository) CanjeadosEntre(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time) (int64, error) {
	return r.cartera.CountDocuments(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "canjeado_en", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}},
	})
}

// --- ARCO ---

// BorrarCarteras borra la cartera de la persona; con estados, solo esos.
func (r *Repository) BorrarCarteras(ctx context.Context, usuario bson.ObjectID, estados ...string) error {
	filter := bson.D{{Key: "usuario_id", Value: usuario}}
	if len(estados) > 0 {
		filter = append(filter, bson.E{Key: "estado", Value: bson.D{{Key: "$in", Value: estados}}})
	}
	_, err := r.cartera.DeleteMany(ctx, filter)
	return err
}

// OlvidarValidador quita `validado_por` de los canjes que validó la persona.
func (r *Repository) OlvidarValidador(ctx context.Context, usuario bson.ObjectID) error {
	_, err := r.cartera.UpdateMany(ctx, bson.D{{Key: "validado_por", Value: usuario}}, bson.D{{Key: "$unset", Value: bson.D{{Key: "validado_por", Value: ""}}}})
	return err
}

// CarterasDe trae toda la cartera de la persona (exportación).
func (r *Repository) CarterasDe(ctx context.Context, usuario bson.ObjectID) ([]Cartera, error) {
	return findAll[Cartera](ctx, r.cartera, bson.D{{Key: "usuario_id", Value: usuario}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}))
}
