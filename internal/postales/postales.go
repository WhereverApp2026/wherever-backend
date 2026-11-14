package postales

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// imagenPropia valida la imagen: ajena o inexistente = 404; sin publicar ni en revisión = 422.
func (s *Service) imagenPropia(ctx context.Context, me bson.ObjectID, id string) (imagenes.Vista, error) {
	if s.imagenes == nil {
		return imagenes.Vista{}, errSinImagenes()
	}
	v, err := s.imagenes.Consultar(ctx, me, id)
	if errors.Is(err, imagenes.ErrNotFound) {
		return v, apperr.NotFound("imagen no encontrada")
	}
	if err != nil {
		return v, errStore()
	}
	if !ImagenUsable(v.Estado) {
		return v, invalid("imagen_id", "la imagen debe estar confirmada y no rechazada")
	}
	return v, nil
}

// negocioVisible valida el negocio de la postal: no aprobado o inexistente = 404.
func (s *Service) negocioVisible(ctx context.Context, raw string) (*bson.ObjectID, error) {
	if raw == "" {
		return nil, nil
	}
	id, _ := parseID(raw)
	_, ok, err := s.Negocios.FindVisible(ctx, id)
	if err != nil {
		return nil, errStore()
	}
	if !ok {
		return nil, errNegocioNotFound()
	}
	return &id, nil
}

// planPropio valida el plan de la postal o el momento: propio o de un grupo donde está
// confirmado; si no, 404.
func (s *Service) planPropio(ctx context.Context, me bson.ObjectID, raw string) (*bson.ObjectID, error) {
	if raw == "" {
		return nil, nil
	}
	id, _ := parseID(raw)
	if _, err := s.planVisible(ctx, me, id); err != nil {
		return nil, err
	}
	return &id, nil
}

// CrearPostal guarda la postal con una imagen propia. Responde 202: la imagen o el texto pueden
// seguir en moderación. Devuelve el id para la cabecera `Location`.
func (s *Service) CrearPostal(ctx context.Context, cl *middleware.Claims, req CrearPostalRequest) (string, error) {
	me, err := self(cl)
	if err != nil {
		return "", err
	}
	img, err := s.imagenPropia(ctx, me, req.ImagenID)
	if err != nil {
		return "", err
	}
	plan, err := s.planPropio(ctx, me, req.PlanID)
	if err != nil {
		return "", err
	}
	negocio, err := s.negocioVisible(ctx, req.NegocioID)
	if err != nil {
		return "", err
	}
	texto := strings.TrimSpace(req.Texto)
	textoEstado, res, err := s.textoEstado(ctx, "texto", texto)
	if err != nil {
		return "", err
	}
	now := s.ahora()
	p := Postal{
		ID: bson.NewObjectID(), UsuarioID: me, ImagenID: req.ImagenID, ImagenEstado: img.Estado, ImagenURL: img.URL,
		PlanID: plan, NegocioID: negocio, Estilo: req.Estilo, Texto: texto, TextoEstado: textoEstado, Visibilidad: req.Visibilidad,
		Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	p.Estado = EstadoPostal(p.ImagenEstado, p.TextoEstado)
	if err := s.Repo.InsertarPostal(ctx, p); errors.Is(err, ErrDuplicado) {
		return "", apperr.Conflict("la imagen ya es otra postal")
	} else if err != nil {
		return "", errStore()
	}
	if textoEstado == TextoEnRevision {
		s.encolar(ctx, TipoColaPostal, p.ID, me, texto, res)
	}
	s.Log.InfoContext(ctx, "postal creada", slog.String("evento", "postal_creada"), slog.String("id", p.ID.Hex()),
		slog.String("estilo", p.Estilo), slog.String("visibilidad", p.Visibilidad), slog.String("estado", p.Estado))
	return p.ID.Hex(), nil
}

// respuestaPostal arma la postal para quien mira: `imagen_id` y `plan_id` solo para el dueño.
func respuestaPostal(p Postal, propia bool, autor *AutorView) PostalResponse {
	out := PostalResponse{
		ID: p.ID.Hex(), Autor: autor, ImagenURL: p.ImagenURL, Estilo: p.Estilo, Texto: p.Texto, Visibilidad: p.Visibilidad,
		Estado: p.Estado, NegocioID: hexPtr(p.NegocioID), Propia: propia, Version: p.Version,
		CreadoEn: p.CreadoEn.UTC(), ActualizadoEn: p.ActualizadoEn.UTC(),
	}
	if propia {
		out.ImagenID, out.PlanID = p.ImagenID, hexPtr(p.PlanID)
	}
	if p.ImagenEstado != imagenes.EstadoPublicada {
		out.ImagenURL = ""
	}
	return out
}

// Album pagina las postales propias con filtros.
func (s *Service) Album(ctx context.Context, cl *middleware.Claims, q AlbumQuery) (pagination.PageResult[PostalResponse], error) {
	var vacio pagination.PageResult[PostalResponse]
	me, err := self(cl)
	if err != nil {
		return vacio, err
	}
	f := FiltroAlbum{Estilo: q.Estilo}
	if q.PlanID != "" {
		id, _ := parseID(q.PlanID)
		f.PlanID = &id
	}
	if q.NegocioID != "" {
		id, _ := parseID(q.NegocioID)
		f.NegocioID = &id
	}
	if q.Mes != "" {
		f.Desde, f.Hasta, _ = RangoMes(q.Mes, s.loc)
	}
	scope := s.Cursor.Scope("postales_album", me.Hex(), q.PlanID, q.NegocioID, q.Estilo, q.Mes)
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.Album(ctx, me, f, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	rows, next, err := pagina(s, rows, q.Limite, scope, func(p Postal) (bson.ObjectID, time.Time) { return p.ID, p.CreadoEn })
	if err != nil {
		return vacio, err
	}
	items := make([]PostalResponse, 0, len(rows))
	for _, p := range rows {
		items = append(items, respuestaPostal(p, true, nil))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// visible trae la postal si quien mira puede verla; si no, 404 (no revela que existe).
func (s *Service) visible(ctx context.Context, viewer *bson.ObjectID, raw string) (Postal, Relacion, error) {
	id, ok := parseID(raw)
	if !ok {
		return Postal{}, Relacion{}, errPostalNotFound()
	}
	p, err := s.Repo.Postal(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return Postal{}, Relacion{}, errPostalNotFound()
	}
	if err != nil {
		return Postal{}, Relacion{}, errStore()
	}
	rel, err := s.relacion(ctx, viewer, p.UsuarioID)
	if err != nil {
		return Postal{}, Relacion{}, err
	}
	if !PuedeVer(rel, p.Visibilidad, p.Estado == EstadoLista) {
		return Postal{}, Relacion{}, errPostalNotFound()
	}
	return p, rel, nil
}

// Postal es el detalle: el dueño siempre; los demás según la visibilidad y solo `lista`.
func (s *Service) Postal(ctx context.Context, cl *middleware.Claims, raw string) (PostalResponse, error) {
	me, err := self(cl)
	if err != nil {
		return PostalResponse{}, err
	}
	p, rel, err := s.visible(ctx, &me, raw)
	if err != nil {
		return PostalResponse{}, err
	}
	if rel.Dueno {
		return respuestaPostal(p, true, nil), nil
	}
	autores, err := s.autores(ctx, []bson.ObjectID{p.UsuarioID})
	if err != nil {
		return PostalResponse{}, err
	}
	autor, ok := autores[p.UsuarioID]
	if !ok {
		return PostalResponse{}, errPostalNotFound() // cuenta no activa
	}
	return respuestaPostal(p, false, &autor), nil
}

// propia trae la postal del dueño; ajena o inexistente = 404.
func (s *Service) propia(ctx context.Context, me bson.ObjectID, raw string) (Postal, error) {
	id, ok := parseID(raw)
	if !ok {
		return Postal{}, errPostalNotFound()
	}
	p, err := s.Repo.Postal(ctx, id)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && p.UsuarioID != me) {
		return Postal{}, errPostalNotFound()
	}
	if err != nil {
		return Postal{}, errStore()
	}
	return p, nil
}

// EditarPostal cambia imagen, estilo, texto o visibilidad con If-Match. Un texto nuevo vuelve a
// moderarse. La postal no se cierra por debajo de la visibilidad de su momento (409).
func (s *Service) EditarPostal(ctx context.Context, cl *middleware.Claims, raw string, version int, req EditarPostalRequest) (PostalResponse, error) {
	me, err := self(cl)
	if err != nil {
		return PostalResponse{}, err
	}
	p, err := s.propia(ctx, me, raw)
	if err != nil {
		return PostalResponse{}, err
	}
	if p.Version != version {
		return PostalResponse{}, errStale()
	}
	nueva, res, err := s.editada(ctx, me, p, req)
	if err != nil {
		return PostalResponse{}, err
	}
	nueva.Estado = EstadoPostal(nueva.ImagenEstado, nueva.TextoEstado)
	out, err := s.Repo.ActualizarPostal(ctx, p.ID, me, version, bson.D{
		{Key: "imagen_id", Value: nueva.ImagenID}, {Key: "imagen_estado", Value: nueva.ImagenEstado}, {Key: "imagen_url", Value: nueva.ImagenURL},
		{Key: "estilo", Value: nueva.Estilo}, {Key: "texto", Value: nueva.Texto}, {Key: "texto_estado", Value: nueva.TextoEstado},
		{Key: "visibilidad", Value: nueva.Visibilidad}, {Key: "estado", Value: nueva.Estado}, {Key: "actualizado_en", Value: s.ahora()},
	})
	switch {
	case errors.Is(err, ErrNoAplica):
		return PostalResponse{}, errStale()
	case errors.Is(err, ErrDuplicado):
		return PostalResponse{}, apperr.Conflict("la imagen ya es otra postal")
	case err != nil:
		return PostalResponse{}, errStore()
	}
	if nueva.Texto != p.Texto && nueva.TextoEstado == TextoEnRevision {
		s.encolar(ctx, TipoColaPostal, p.ID, me, nueva.Texto, res)
	}
	return respuestaPostal(out, true, nil), nil
}

// editada aplica el cuerpo y valida imagen nueva, texto y visibilidad frente al momento.
func (s *Service) editada(ctx context.Context, me bson.ObjectID, p Postal, req EditarPostalRequest) (Postal, moderacion.Resultado, error) {
	var res moderacion.Resultado
	nueva, err := s.nuevaImagen(ctx, me, p, req.ImagenID)
	if err != nil {
		return p, res, err
	}
	if req.Estilo != nil {
		nueva.Estilo = *req.Estilo
	}
	if req.Texto != nil && strings.TrimSpace(*req.Texto) != p.Texto {
		nueva.Texto = strings.TrimSpace(*req.Texto)
		estado, r, err := s.textoEstado(ctx, "texto", nueva.Texto)
		if err != nil {
			return p, res, err
		}
		nueva.TextoEstado, res = estado, r
	}
	if req.Visibilidad != nil && *req.Visibilidad != p.Visibilidad {
		nueva.Visibilidad = *req.Visibilidad
		if err := s.cabeMomento(ctx, p.ID, nueva.Visibilidad); err != nil {
			return p, res, err
		}
	}
	return nueva, res, nil
}

// nuevaImagen cambia la imagen de la postal si el PATCH trae otra (la app dibujó otro estilo).
func (s *Service) nuevaImagen(ctx context.Context, me bson.ObjectID, p Postal, id *string) (Postal, error) {
	if id == nil || *id == p.ImagenID {
		return p, nil
	}
	img, err := s.imagenPropia(ctx, me, *id)
	if err != nil {
		return p, err
	}
	p.ImagenID, p.ImagenEstado, p.ImagenURL = *id, img.Estado, img.URL
	return p, nil
}

// cabeMomento revisa que la nueva visibilidad de la postal no quede por debajo de su momento.
func (s *Service) cabeMomento(ctx context.Context, postal bson.ObjectID, vis string) error {
	m, err := s.Repo.MomentoDePostal(ctx, postal)
	if errors.Is(err, ErrNoEncontrado) {
		return nil
	}
	if err != nil {
		return errStore()
	}
	if vis == VisPrivada || !Compatible(vis, m.Visibilidad) {
		return apperr.Conflict("la postal tiene un momento más abierto; cámbialo o bórralo antes")
	}
	return nil
}

// BorrarPostal borra la postal con If-Match y su momento. La imagen se queda (es de nv-45).
func (s *Service) BorrarPostal(ctx context.Context, cl *middleware.Claims, raw string, version int) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	p, err := s.propia(ctx, me, raw)
	if err != nil {
		return err
	}
	if p.Version != version {
		return errStale()
	}
	if err := s.Repo.BorrarPostal(ctx, p.ID, me, version); errors.Is(err, ErrNoAplica) {
		return errStale()
	} else if err != nil {
		return errStore()
	}
	s.Log.InfoContext(ctx, "postal borrada", slog.String("evento", "postal_borrada"), slog.String("id", p.ID.Hex()))
	return nil
}

// Galeria es la galería pública de un lugar: solo `publica` y `lista`, de cuentas activas y sin
// las de personas bloqueadas con quien mira. Negocio no aprobado = 404.
func (s *Service) Galeria(ctx context.Context, cl *middleware.Claims, raw string, q ListaQuery) (pagination.PageResult[PostalResponse], error) {
	var vacio pagination.PageResult[PostalResponse]
	id, ok := parseID(raw)
	if !ok {
		return vacio, errNegocioNotFound()
	}
	if _, err := s.negocioVisible(ctx, id.Hex()); err != nil {
		return vacio, err
	}
	scope := s.Cursor.Scope("postales_galeria", id.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.Galeria(ctx, id, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	rows, next, err := pagina(s, rows, q.Limite, scope, func(p Postal) (bson.ObjectID, time.Time) { return p.ID, p.CreadoEn })
	if err != nil {
		return vacio, err
	}
	items, err := s.itemsGaleria(ctx, quien(cl), rows)
	if err != nil {
		return vacio, err
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// itemsGaleria quita las postales de cuentas no activas o bloqueadas con quien mira.
func (s *Service) itemsGaleria(ctx context.Context, viewer *bson.ObjectID, rows []Postal) ([]PostalResponse, error) {
	bloq, err := s.bloqueados(ctx, viewer)
	if err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.UsuarioID)
	}
	autores, err := s.autores(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]PostalResponse, 0, len(rows))
	for _, p := range rows {
		autor, activo := autores[p.UsuarioID]
		if !activo || bloq[p.UsuarioID] {
			continue
		}
		propia := viewer != nil && *viewer == p.UsuarioID
		items = append(items, respuestaPostal(p, propia, &autor))
	}
	return items, nil
}
