package cupones

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Con `q` la cartera responde una sola página filtrada en memoria (la cartera es pequeña).
const topeBusqueda = 200

// Motivos del 409 al canjear.
const (
	MotivoVencido    = "vencido"
	MotivoFuera      = "fuera_de_horario"
	MotivoAgotado    = "agotado"
	MotivoYaCanjeado = "ya_canjeado"
)

// visibles devuelve los ids de los negocios publicados de la lista.
func (s *Service) visibles(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error) {
	bs, err := s.negocios.FindVisibleMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[bson.ObjectID]bool, len(bs))
	for _, b := range bs {
		out[b.ID] = true
	}
	return out, nil
}

func (s *Service) posCerca(scope, raw string) (*PosCerca, error) {
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	d, errD := strconv.ParseFloat(c.Sort, 64)
	if !okID || errD != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &PosCerca{Distancia: d, ID: id}, nil
}

// Cerca son los cupones vigentes en el radio de negocios publicados, del más cercano al más
// lejano. Público; no cuenta vistas.
func (s *Service) Cerca(ctx context.Context, q CercaQuery) (pagination.PageResult[CuponResponse], error) {
	if s.cursor == nil {
		return pagination.PageResult[CuponResponse]{}, errStore()
	}
	scope := s.cursor.Scope("cupones_cerca", strconv.FormatFloat(*q.Lat, 'f', 6, 64), strconv.FormatFloat(*q.Lng, 'f', 6, 64),
		strconv.FormatFloat(q.RadioKm, 'f', 3, 64), q.Categoria)
	after, err := s.posCerca(scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[CuponResponse]{}, err
	}
	filas, err := s.repo.Cerca(ctx, *q.Lng, *q.Lat, q.RadioKm*1000, q.Categoria, s.now().UTC(), after, q.Limite)
	if err != nil {
		return pagination.PageResult[CuponResponse]{}, errStore()
	}
	next := ""
	if len(filas) > q.Limite {
		filas = filas[:q.Limite]
		last := filas[len(filas)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: strconv.FormatFloat(last.DistanciaM, 'g', -1, 64), Scope: scope}); err != nil {
			return pagination.PageResult[CuponResponse]{}, errStore()
		}
	}
	ids := make([]bson.ObjectID, 0, len(filas))
	for _, f := range filas {
		ids = append(ids, f.NegocioID)
	}
	ok, err := s.visibles(ctx, ids)
	if err != nil {
		return pagination.PageResult[CuponResponse]{}, errStore()
	}
	items := make([]CuponResponse, 0, len(filas))
	for _, f := range filas {
		if !ok[f.NegocioID] {
			continue
		}
		r := publicoResponse(f.Cupon)
		d := f.DistanciaM
		r.DistanciaM = &d
		items = append(items, r)
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// publicado trae el cupón vigente de un negocio publicado; si no, 404.
func (s *Service) publicado(ctx context.Context, raw string) (Cupon, error) {
	id, ok := parseID(raw)
	if !ok {
		return Cupon{}, errCuponNotFound()
	}
	c, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Cupon{}, errCuponNotFound()
	}
	if err != nil {
		return Cupon{}, errStore()
	}
	if !c.Vigente(s.now()) {
		return Cupon{}, errCuponNotFound()
	}
	_, visible, err := s.negocios.FindVisible(ctx, c.NegocioID)
	if err != nil {
		return Cupon{}, errStore()
	}
	if !visible {
		return Cupon{}, errCuponNotFound()
	}
	return c, nil
}

// contar suma un contador del cupón y de su día; si falla solo se registra.
func (s *Service) contar(ctx context.Context, id bson.ObjectID, campoCupon, campoDia string) {
	ctx = context.WithoutCancel(ctx)
	var err error
	if campoCupon != "" {
		err = s.repo.Sumar(ctx, id, campoCupon)
	}
	if err == nil {
		err = s.repo.SumarMetrica(ctx, id, campoDia, s.now())
	}
	if err != nil {
		s.log.Warn("cupones: no se sumó la métrica", "evento", "cupones_metrica_fallo", "campo", campoDia, "error", err.Error())
	}
}

// Detalle es el cupón público; cada apertura suma una vista.
func (s *Service) Detalle(ctx context.Context, raw string) (CuponResponse, error) {
	c, err := s.publicado(ctx, raw)
	if err != nil {
		return CuponResponse{}, err
	}
	s.contar(ctx, c.ID, "vistas", "vistas")
	return publicoResponse(c), nil
}

// Guardar agrega el cupón a la cartera una sola vez (ux_cartera da el 409 sin leer antes).
func (s *Service) Guardar(ctx context.Context, cl *middleware.Claims, raw string) (CarteraResponse, error) {
	me, err := self(cl)
	if err != nil {
		return CarteraResponse{}, err
	}
	c, err := s.publicado(ctx, raw)
	if err != nil {
		return CarteraResponse{}, err
	}
	if c.Agotado() {
		return CarteraResponse{}, apperr.Conflict(MotivoAgotado)
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	w := Cartera{
		ID: bson.NewObjectID(), UsuarioID: me, CuponID: c.ID, NegocioID: c.NegocioID,
		Resumen: Resumen{Titulo: c.Titulo, Tipo: c.Tipo, Valor: c.Valor, NegocioNombre: c.NegocioNombre, ResumenNormalizado: ResumenNormalizado(c.Titulo, c.NegocioNombre)},
		Estado:  CarteraGuardado, VenceEn: c.VigenciaFin, CreadoEn: now, ActualizadoEn: now,
	}
	if err := s.repo.InsertCartera(ctx, &w); errors.Is(err, ErrDuplicate) {
		return CarteraResponse{}, apperr.Conflict("ya_guardado")
	} else if err != nil {
		return CarteraResponse{}, errStore()
	}
	s.contar(ctx, c.ID, "guardados", "guardados")
	return carteraResponse(w, now, false), nil
}

// filtroCartera traduce el filtro: vigentes y por vencer = guardado o en canje sin vencer
// (por vencer, en las próximas 24 h); usados = canjeado, del más reciente al más viejo.
func filtroCartera(filtro string, now time.Time) FiltroCartera {
	switch filtro {
	case "usados":
		return FiltroCartera{Estados: []string{CarteraCanjeado}, Campo: "canjeado_en", Desc: true}
	case "por_vencer":
		hasta := now.Add(24 * time.Hour)
		return FiltroCartera{Estados: []string{CarteraGuardado, CarteraEnCanje}, Desde: &now, Hasta: &hasta, Campo: "vence_en"}
	default:
		return FiltroCartera{Estados: []string{CarteraGuardado, CarteraEnCanje}, Desde: &now, Campo: "vence_en"}
	}
}

func fechaOrden(c Cartera, campo string) time.Time {
	if campo == "canjeado_en" && c.CanjeadoEn != nil {
		return c.CanjeadoEn.UTC()
	}
	return c.VenceEn.UTC()
}

// Mias es la cartera de la persona con filtro y búsqueda por prefijo.
func (s *Service) Mias(ctx context.Context, cl *middleware.Claims, q MisQuery) (pagination.PageResult[CarteraResponse], error) {
	me, err := self(cl)
	if err != nil {
		return pagination.PageResult[CarteraResponse]{}, err
	}
	if s.cursor == nil {
		return pagination.PageResult[CarteraResponse]{}, errStore()
	}
	now := s.now().UTC()
	f := filtroCartera(q.Filtro, now)
	if q.Q != "" {
		return s.buscar(ctx, me, f, q, now)
	}
	scope := s.cursor.Scope("mis_cupones", me.Hex(), q.Filtro)
	var after *PosCartera
	if pos, ok, err := s.posicion(scope, q.Cursor); err != nil {
		return pagination.PageResult[CarteraResponse]{}, err
	} else if ok {
		after = &pos
	}
	cs, err := s.repo.Mias(ctx, me, f, after, q.Limite)
	if err != nil {
		return pagination.PageResult[CarteraResponse]{}, errStore()
	}
	next := ""
	if len(cs) > q.Limite {
		cs = cs[:q.Limite]
		last := cs[len(cs)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: fechaOrden(last, f.Campo).Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return pagination.PageResult[CarteraResponse]{}, errStore()
		}
	}
	items := make([]CarteraResponse, 0, len(cs))
	for _, c := range cs {
		items = append(items, carteraResponse(c, now, q.Filtro == "por_vencer"))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// buscar filtra por prefijo hasta 200 cupones del filtro; una sola página, sin cursor.
func (s *Service) buscar(ctx context.Context, me bson.ObjectID, f FiltroCartera, q MisQuery, now time.Time) (pagination.PageResult[CarteraResponse], error) {
	cs, err := s.repo.Mias(ctx, me, f, nil, topeBusqueda-1)
	if err != nil {
		return pagination.PageResult[CarteraResponse]{}, errStore()
	}
	items := []CarteraResponse{}
	for _, c := range cs {
		if CoincidePrefijo(c.Resumen.ResumenNormalizado, q.Q) {
			items = append(items, carteraResponse(c, now, q.Filtro == "por_vencer"))
		}
	}
	return pagination.NewPageResult(items, "", topeBusqueda), nil
}

// Quitar borra un cupón de la cartera propia; ajeno = 404.
func (s *Service) Quitar(ctx context.Context, cl *middleware.Claims, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	id, ok := parseID(raw)
	if !ok {
		return errCuponNotFound()
	}
	if err := s.repo.QuitarCartera(ctx, me, id); errors.Is(err, ErrNotFound) {
		return errCuponNotFound()
	} else if err != nil {
		return errStore()
	}
	return nil
}

// canjeable revisa la cartera y su cupón antes de dar código; el motivo va en el 409.
func (s *Service) canjeable(ctx context.Context, w Cartera, now time.Time) error {
	switch {
	case w.Estado == CarteraCanjeado:
		return apperr.Conflict(MotivoYaCanjeado)
	case w.Estado == CarteraVencido || !w.VenceEn.After(now):
		return apperr.Conflict(MotivoVencido)
	}
	c, err := s.repo.FindByID(ctx, w.CuponID)
	if errors.Is(err, ErrNotFound) {
		return apperr.Conflict(MotivoVencido)
	}
	if err != nil {
		return errStore()
	}
	if _, visible, err := s.negocios.FindVisible(ctx, c.NegocioID); err != nil {
		return errStore()
	} else if !visible {
		return errCuponNotFound()
	}
	return motivoCupon(c, now, s.loc)
}

// motivoCupon da el 409 si el cupón no se puede usar ahora (agotado, vencido o fuera de horario).
func motivoCupon(c Cupon, now time.Time, loc *time.Location) error {
	switch {
	case c.Agotado():
		return apperr.Conflict(MotivoAgotado)
	case !c.Vigente(now):
		return apperr.Conflict(MotivoVencido)
	case !EnHorario(c.DiasValidos, c.HorasValidas, now, loc):
		return apperr.Conflict(MotivoFuera)
	}
	return nil
}

// codigoVivo dice si la cartera ya tiene un código que no expiró.
func codigoVivo(w Cartera, now time.Time) bool {
	return w.Estado == CarteraEnCanje && w.CodigoCanje != "" && w.CanjeExpiraEn != nil && w.CanjeExpiraEn.After(now)
}

// Canjear da el código de un solo uso (5 min). Mientras vive, pedirlo otra vez devuelve el
// mismo; un choque con ux_codigo_canje se reintenta hasta 3 veces.
func (s *Service) Canjear(ctx context.Context, cl *middleware.Claims, raw string) (CanjeResponse, error) {
	me, err := self(cl)
	if err != nil {
		return CanjeResponse{}, err
	}
	id, ok := parseID(raw)
	if !ok {
		return CanjeResponse{}, errCuponNotFound()
	}
	w, err := s.repo.CarteraDe(ctx, me, id)
	if errors.Is(err, ErrNotFound) {
		return CanjeResponse{}, errCuponNotFound()
	}
	if err != nil {
		return CanjeResponse{}, errStore()
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	if codigoVivo(w, now) {
		return canjeResponse(w), nil
	}
	if err := s.canjeable(ctx, w, now); err != nil {
		return CanjeResponse{}, err
	}
	return s.nuevoCodigo(ctx, me, id, now)
}

func (s *Service) nuevoCodigo(ctx context.Context, me, id bson.ObjectID, now time.Time) (CanjeResponse, error) {
	for range 3 {
		codigo, err := s.codigo()
		if err != nil {
			return CanjeResponse{}, errStore()
		}
		w, err := s.repo.PonerCodigo(ctx, me, id, codigo, now.Add(VidaCodigo), now)
		switch {
		case err == nil:
			return canjeResponse(w), nil
		case errors.Is(err, ErrDuplicate):
			continue
		case errors.Is(err, ErrNoAplica):
			// Otra petición lo cambió a la vez: si dejó un código vivo, es el mismo.
			if w, errW := s.repo.CarteraDe(ctx, me, id); errW == nil && codigoVivo(w, now) {
				return canjeResponse(w), nil
			}
			return CanjeResponse{}, apperr.Conflict(MotivoVencido)
		default:
			return CanjeResponse{}, errStore()
		}
	}
	return CanjeResponse{}, errStore()
}
