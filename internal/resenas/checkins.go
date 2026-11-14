package resenas

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// MensajeNoLlegas es el texto neutro del Excel para un check-in no válido: no acusa.
const MensajeNoLlegas = "Parece que aún no llegas. ¿Escaneas el código?"

func errNoLlegas() error {
	return apperr.Validation(MensajeNoLlegas, apperr.FieldError{Field: "checkin", Message: MensajeNoLlegas})
}

// Checkin registra la llegada al negocio. Uno no válido se guarda (sin coordenadas) y responde
// 422; el segundo válido del día responde 409 sin guardar otro documento.
func (s *Service) Checkin(ctx context.Context, cl *middleware.Claims, req CheckinRequest) (CheckinResponse, error) {
	me, err := self(cl)
	if err != nil {
		return CheckinResponse{}, err
	}
	nid, _ := parseID(req.NegocioID)
	b, ok, err := s.negocios.FindVisible(ctx, nid)
	if err != nil {
		return CheckinResponse{}, errStore()
	}
	if !ok {
		return CheckinResponse{}, errNegocioNotFound()
	}
	now := s.ahora()
	c := Checkin{ID: bson.NewObjectID(), UsuarioID: me, NegocioID: b.ID, Metodo: req.Metodo, Dia: Dia(now, s.loc), CreadoEn: now}
	if pid, ok := parseID(req.PlanID); ok {
		c.PlanID = &pid
	}
	if req.Metodo == MetodoUbicacion {
		c.MotivoRechazo, err = s.porUbicacion(ctx, me, b, req, &c, now)
	} else {
		c.MotivoRechazo, err = s.porQR(ctx, b, req.Codigo)
	}
	if err != nil {
		return CheckinResponse{}, err
	}
	return s.guardarCheckin(ctx, c)
}

// guardarCheckin guarda el check-in: el válido repetido del día = 409; el no válido queda
// registrado (sin coordenadas) y responde 422 neutro.
func (s *Service) guardarCheckin(ctx context.Context, c Checkin) (CheckinResponse, error) {
	c.Valido = c.MotivoRechazo == ""
	switch err := s.repo.InsertarCheckin(ctx, c); {
	case errors.Is(err, ErrDuplicado):
		return CheckinResponse{}, apperr.Conflict("ya registraste tu llegada a este negocio hoy")
	case err != nil:
		return CheckinResponse{}, errStore()
	}
	if c.Valido {
		return checkinResponse(c), nil
	}
	if c.MotivoRechazo == MotivoUbicacionFalsa {
		s.log.InfoContext(ctx, "check-in con salto imposible", slog.String("evento", "checkin_ubicacion_falsa"),
			slog.String("checkin_id", c.ID.Hex()), slog.String("negocio_id", c.NegocioID.Hex()))
	}
	return CheckinResponse{}, errNoLlegas()
}

// porUbicacion revisa radio, precisión y salto imposible. Solo guarda la distancia y la precisión.
func (s *Service) porUbicacion(ctx context.Context, me bson.ObjectID, b negocios.Business, req CheckinRequest, c *Checkin, now time.Time) (string, error) {
	lat, lng, prec := *req.Lat, *req.Lng, *req.PrecisionM
	if len(b.Location.Coordinates) < 2 {
		return MotivoFueraDeRadio, nil
	}
	dist := DistanciaM(lat, lng, b.Location.Coordinates[1], b.Location.Coordinates[0])
	c.DistanciaM, c.PrecisionM = &dist, &prec
	if motivo := MotivoUbicacion(dist, prec); motivo != "" {
		return motivo, nil
	}
	return s.salto(ctx, me, b, now)
}

// salto revisa la velocidad entre el negocio del último check-in válido y este.
func (s *Service) salto(ctx context.Context, me bson.ObjectID, b negocios.Business, now time.Time) (string, error) {
	ultimo, ok, err := s.repo.UltimoCheckinValido(ctx, me)
	if err != nil {
		return "", errStore()
	}
	if !ok || ultimo.NegocioID == b.ID {
		return "", nil
	}
	antes, err := s.negocios.FindByID(ctx, ultimo.NegocioID)
	if err != nil && !errors.Is(err, negocios.ErrNotFound) {
		return "", errStore()
	}
	if err != nil || len(antes.Location.Coordinates) < 2 {
		return "", nil
	}
	km := planes.Haversine(antes.Location.Coordinates[1], antes.Location.Coordinates[0], b.Location.Coordinates[1], b.Location.Coordinates[0])
	if SaltoImposible(km, now.Sub(ultimo.CreadoEn)) {
		return MotivoUbicacionFalsa, nil
	}
	return "", nil
}

// porQR valida el código del mostrador: debe ser del mismo negocio. Otro negocio responde igual
// que fuera de radio, sin revelar de cuál es.
func (s *Service) porQR(ctx context.Context, b negocios.Business, codigo string) (string, error) {
	dueno, ok, err := s.negocios.FindVisibleByQR(ctx, codigo)
	if err != nil {
		return "", errStore()
	}
	if !ok || dueno.ID != b.ID {
		return MotivoFueraDeRadio, nil
	}
	return "", nil
}

func checkinResponse(c Checkin) CheckinResponse {
	out := CheckinResponse{ID: c.ID.Hex(), NegocioID: c.NegocioID.Hex(), Metodo: c.Metodo, Valido: c.Valido, DistanciaM: c.DistanciaM, Dia: c.Dia, CreadoEn: c.CreadoEn.UTC()}
	if c.ResenaID != nil {
		r := c.ResenaID.Hex()
		out.ResenaID = &r
	}
	return out
}

func (s *Service) posicion(scope, raw string) (*Pos, error) {
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	t, errT := time.Parse(time.RFC3339Nano, c.Sort)
	if !okID || errT != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Pos{Fecha: t, ID: id}, nil
}

func (s *Service) siguiente(scope string, id bson.ObjectID, sort string) (string, error) {
	next, err := s.cursor.Encode(pagination.Cursor{ID: id.Hex(), Sort: sort, Scope: scope})
	if err != nil {
		return "", errStore()
	}
	return next, nil
}

// Checkins pagina los check-ins de la persona, válidos o no.
func (s *Service) Checkins(ctx context.Context, cl *middleware.Claims, q ListaQuery) (pagination.PageResult[CheckinResponse], error) {
	me, err := self(cl)
	if err != nil {
		return pagination.PageResult[CheckinResponse]{}, err
	}
	scope := s.cursor.Scope("checkins", me.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[CheckinResponse]{}, err
	}
	cs, err := s.repo.MisCheckins(ctx, me, after, q.Limite)
	if err != nil {
		return pagination.PageResult[CheckinResponse]{}, errStore()
	}
	next := ""
	if len(cs) > q.Limite {
		cs = cs[:q.Limite]
		last := cs[len(cs)-1]
		if next, err = s.siguiente(scope, last.ID, last.CreadoEn.UTC().Format(time.RFC3339Nano)); err != nil {
			return pagination.PageResult[CheckinResponse]{}, err
		}
	}
	items := make([]CheckinResponse, 0, len(cs))
	for _, c := range cs {
		items = append(items, checkinResponse(c))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// Pendientes es «Por reseñar»: visitas válidas sin reseña de los últimos 30 días.
func (s *Service) Pendientes(ctx context.Context, cl *middleware.Claims) (PendientesResponse, error) {
	me, err := self(cl)
	if err != nil {
		return PendientesResponse{}, err
	}
	cs, err := s.repo.PorResenar(ctx, me, s.ahora().AddDate(0, 0, -DiasPorResenar), 50)
	if err != nil {
		return PendientesResponse{}, errStore()
	}
	ids := make([]bson.ObjectID, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.NegocioID)
	}
	nombres := map[bson.ObjectID]string{}
	if len(ids) > 0 {
		bs, err := s.negocios.FindVisibleMany(ctx, ids)
		if err != nil {
			return PendientesResponse{}, errStore()
		}
		for _, b := range bs {
			nombres[b.ID] = b.Name
		}
	}
	out := PendientesResponse{Items: []PendienteResponse{}}
	for _, c := range cs {
		nombre, ok := nombres[c.NegocioID]
		if !ok {
			continue // el negocio ya no está visible
		}
		out.Items = append(out.Items, PendienteResponse{
			CheckinID: c.ID.Hex(), NegocioID: c.NegocioID.Hex(), NegocioNombre: nombre, Dia: c.Dia,
			VenceEn: c.CreadoEn.AddDate(0, 0, DiasPorResenar).UTC(),
		})
	}
	return out, nil
}
