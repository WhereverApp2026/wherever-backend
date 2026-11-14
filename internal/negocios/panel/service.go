package panel

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/menu"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Límites del panel (spec nv-56).
const (
	MaxPausa     = 90 * 24 * time.Hour
	PlazoFuente  = time.Second
	esquemaQR    = "wherever://checkin/"
	zonaNegocios = "America/Mexico_City"
)

// Expediente es lo que el panel usa de nv-55: si los fiscales están completos (sin leerlos) y
// la cancelación del expediente al dar de baja.
type Expediente interface {
	Vigentes(ctx context.Context, negocio bson.ObjectID) (map[string]bool, error)
	CancelarNegocio(ctx context.Context, negocio bson.ObjectID) error
}

// Imagenes borra las imágenes de nv-45 del negocio al darlo de baja.
type Imagenes interface {
	Eliminar(ctx context.Context, sujeto, id string) error
}

// Service es el panel del negocio; la propiedad se valida aquí.
type Service struct {
	repo         *negocios.Repository
	menu         *menu.Repository
	exp          Expediente
	fichas       alta.Fichas
	observadores negocios.Observadores
	fuentes      negocios.PanelFuente
	futuras      negocios.ReservasFuturas
	imagenes     Imagenes
	notifier     notificaciones.Notifier
	zona         *time.Location
	plazo        time.Duration
	log          *slog.Logger
	now          func() time.Time
}

// NewService recibe sus dependencias por constructor. Las fuentes del panel, los observadores,
// las reservas futuras y las imágenes se conectan con los With*.
func NewService(repo *negocios.Repository, m *menu.Repository, exp Expediente, fichas alta.Fichas, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	zona, err := time.LoadLocation(zonaNegocios)
	if err != nil {
		zona = time.UTC
	}
	return &Service{repo: repo, menu: m, exp: exp, fichas: fichas, notifier: notificaciones.Noop{}, zona: zona, plazo: PlazoFuente, log: log, now: time.Now}
}

// WithObservadores registra quién se entera de pausas y bajas (nv-59, nv-61).
func (s *Service) WithObservadores(o negocios.Observadores) *Service {
	s.observadores = o
	return s
}

// WithFuentes registra las tarjetas del panel (nv-59, nv-61, nv-62).
func (s *Service) WithFuentes(f negocios.PanelFuente) *Service {
	s.fuentes = f
	return s
}

// WithReservasFuturas conecta la consulta de nv-59 que frena la baja.
func (s *Service) WithReservasFuturas(r negocios.ReservasFuturas) *Service {
	s.futuras = r
	return s
}

// WithImagenes conecta el borrado de imágenes de nv-45.
func (s *Service) WithImagenes(i Imagenes) *Service {
	s.imagenes = i
	return s
}

// WithNotifier conecta el aviso al dueño (Sello).
func (s *Service) WithNotifier(n notificaciones.Notifier) *Service {
	if n != nil {
		s.notifier = n
	}
	return s
}

// WithPlazoFuente cambia el tiempo de cada fuente del panel (pruebas).
func (s *Service) WithPlazoFuente(d time.Duration) *Service {
	s.plazo = d
	return s
}

func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errNoEncontrado() error { return apperr.NotFound("negocio no encontrado") }
func errEstado() error       { return apperr.Conflict("estado_no_permite") }

func invalido(campo, msg string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: msg})
}

func sujeto(claims *middleware.Claims) (bson.ObjectID, error) {
	if claims == nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	id, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("sesión inválida")
	}
	return id, nil
}

func (s *Service) buscar(ctx context.Context, raw string) (negocios.Business, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return negocios.Business{}, errNoEncontrado()
	}
	b, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) || (err == nil && b.DeletedAt != nil) {
		return negocios.Business{}, errNoEncontrado()
	}
	if err != nil {
		return negocios.Business{}, errStore()
	}
	return b, nil
}

// propio trae el negocio de la persona sin baja. Uno ajeno da 404 para no confirmar que existe.
func (s *Service) propio(ctx context.Context, claims *middleware.Claims, raw string) (negocios.Business, error) {
	owner, err := sujeto(claims)
	if err != nil {
		return negocios.Business{}, err
	}
	b, err := s.buscar(ctx, raw)
	if err != nil {
		return negocios.Business{}, err
	}
	if b.OwnerID != owner {
		return negocios.Business{}, errNoEncontrado()
	}
	return b, nil
}

var publicados = []string{negocios.StatusApproved, negocios.StatusPaused}

func noEliminado() bson.E {
	return bson.E{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}}
}

// despues invalida la ficha y avisa a los observadores; un observador que falla deja el aviso
// pendiente para el trabajo nocturno y no revierte nada.
func (s *Service) despues(ctx context.Context, id bson.ObjectID, evento string) {
	ctx = context.WithoutCancel(ctx)
	if s.fichas != nil {
		s.fichas.InvalidarFicha(ctx, id.Hex())
	}
	if evento == "" || len(s.observadores) == 0 {
		return
	}
	if err := s.observadores.Avisar(ctx, evento, id); err != nil {
		s.log.WarnContext(ctx, "panel: observador sin avisar; se reintenta de noche", "evento", "observador_fallido", "aviso", evento)
		if err := s.repo.MarcarAviso(ctx, id, evento); err != nil {
			s.log.ErrorContext(ctx, "panel: aviso pendiente sin guardar", "evento", "aviso_sin_guardar")
		}
	}
}

// Panel es el resumen del día (P-47). Cada fuente tiene su plazo; una caída deja null.
func (s *Service) Panel(ctx context.Context, claims *middleware.Claims, id string) (PanelResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return PanelResponse{}, err
	}
	now := s.now()
	dia := s.hoy(now)
	out := PanelResponse{NegocioID: b.ID.Hex(), Nombre: b.Name, Estado: b.State, Pausado: pausaResponse(b.Paused), SelloExcelencia: b.ExcellenceSeal}
	if c := b.CambiosPendientes; c != nil {
		out.CambiosPendientes = &CambiosResumen{Campos: nonNil(c.Campos), EnviadoEn: c.EnviadoEn.UTC()}
	}
	if c := b.CuentaDeposito; c != nil {
		out.CuentaDeposito = &CuentaResponse{CLABEUltimos4: c.Ultimos4, Banco: c.Banco, Estado: c.Estado}
	}
	var (
		reservas *negocios.ResumenReservas
		cupones  *negocios.ResumenCupones
		resenas  *negocios.ResumenResenas
		fiscales *bool
	)
	var grupo sync.WaitGroup
	if f := s.fuentes.Reservas; f != nil {
		grupo.Go(func() {
			reservas = consultar(ctx, s.plazo, func(c context.Context) (negocios.ResumenReservas, error) { return f.ResumenReservas(c, b.ID, dia) })
		})
	}
	if f := s.fuentes.Cupones; f != nil {
		grupo.Go(func() {
			cupones = consultar(ctx, s.plazo, func(c context.Context) (negocios.ResumenCupones, error) { return f.ResumenCupones(c, b.ID, dia) })
		})
	}
	if f := s.fuentes.Resenas; f != nil {
		grupo.Go(func() {
			resenas = consultar(ctx, s.plazo, func(c context.Context) (negocios.ResumenResenas, error) { return f.ResumenResenas(c, b.ID, dia) })
		})
	}
	if s.exp != nil {
		grupo.Go(func() {
			vigentes := consultar(ctx, s.plazo, func(c context.Context) (map[string]bool, error) { return s.exp.Vigentes(c, b.ID) })
			if vigentes != nil {
				ok := (*vigentes)[expediente.TipoFiscales]
				fiscales = &ok
			}
		})
	}
	grupo.Wait()
	out.DatosFiscalesCompletos = fiscales
	llenarTarjetas(&out, reservas, cupones, resenas)
	aprobado := b.Revision.RevisadoEn
	if !slices.Contains(publicados, b.State) {
		aprobado = nil
	}
	out.AvanceSello = Avance(aprobado, now, resenas)
	return out, nil
}

func llenarTarjetas(out *PanelResponse, reservas *negocios.ResumenReservas, cupones *negocios.ResumenCupones, resenas *negocios.ResumenResenas) {
	if reservas != nil {
		out.ReservasHoy = &reservas.Hoy
		if p := reservas.Proxima; p != nil {
			out.ProximaReserva = &ProximaResponse{Hora: p.Hora.UTC(), Personas: p.Personas}
		}
	}
	if cupones != nil {
		out.CuponesCanjeadosHoy, out.CuponesActivos = &cupones.CanjeadosHoy, &cupones.Activos
	}
	if resenas != nil {
		out.LlegadasHoy, out.ResenasPorResponder = &resenas.LlegadasHoy, &resenas.PorResponder
		out.CalificacionSemana, out.CambioVsSemanaAnterior = &resenas.CalificacionSemana, &resenas.CambioVsSemanaAnterior
	}
}

// hoy es el día en curso en la zona de los negocios (constitución: la de la ciudad, con
// America/Mexico_City por defecto).
func (s *Service) hoy(now time.Time) negocios.Periodo {
	local := now.In(s.zona)
	desde := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.zona)
	return negocios.Periodo{Desde: desde.UTC(), Hasta: desde.AddDate(0, 0, 1).UTC()}
}

// consultar corre fn con su plazo; un error o el plazo vencido dan nil.
func consultar[T any](ctx context.Context, plazo time.Duration, fn func(context.Context) (T, error)) *T {
	ctx, cancel := context.WithTimeout(ctx, plazo)
	defer cancel()
	type resultado struct {
		v   T
		err error
	}
	ch := make(chan resultado, 1)
	go func() {
		v, err := fn(ctx)
		ch <- resultado{v, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil
		}
		return &r.v
	case <-ctx.Done():
		return nil
	}
}

var telefonoValido = regexp.MustCompile(`^\+?[0-9 ]{8,20}$`)

// Perfil aplica al momento lo de los pasos 1, 3 y 5 del alta; nombre, dirección y ubicación
// van a `cambios_pendientes` y el negocio sigue publicado con los de antes (If-Match).
func (s *Service) Perfil(ctx context.Context, claims *middleware.Claims, id string, version int, req PerfilRequest) (PerfilResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return PerfilResponse{}, err
	}
	if !slices.Contains(publicados, b.State) {
		return PerfilResponse{}, errEstado()
	}
	set, fields := perfilSet(b, req)
	pend, pfields := pendientes(b, req, s.now().UTC().Truncate(time.Millisecond))
	if fields = append(fields, pfields...); len(fields) > 0 {
		return PerfilResponse{}, apperr.Validation("datos inválidos", fields...)
	}
	if pend != nil {
		set = append(set, bson.E{Key: "cambios_pendientes", Value: pend})
	}
	nb, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "version", Value: version},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: publicados}}}, noEliminado()}, set)
	if errors.Is(err, negocios.ErrNoAplica) {
		return PerfilResponse{}, apperr.Conflict("el negocio cambió; vuelve a cargarlo")
	}
	if err != nil {
		return PerfilResponse{}, errStore()
	}
	s.despues(ctx, nb.ID, "")
	return perfilResponse(nb), nil
}

// perfilSet arma el `$set` de los campos que se aplican al momento y sus errores.
func perfilSet(b negocios.Business, req PerfilRequest) (bson.D, []apperr.FieldError) {
	set, fields := perfilBasico(req)
	oferta, ofields := perfilOferta(b, req)
	return append(set, oferta...), append(fields, ofields...)
}

// perfilBasico son descripción, giro, horarios, accesibilidad y teléfono.
func perfilBasico(req PerfilRequest) (bson.D, []apperr.FieldError) {
	set := bson.D{}
	var fields []apperr.FieldError
	add := func(k string, v any) { set = append(set, bson.E{Key: k, Value: v}) }
	if req.Descripcion != nil {
		add("descripcion", strings.TrimSpace(*req.Descripcion))
	}
	if req.Categoria != nil {
		if !negocios.ValidCategory(*req.Categoria) {
			fields = append(fields, apperr.FieldError{Field: "categoria", Message: "giro no válido"})
		}
		add("categoria", *req.Categoria)
	}
	if req.Horarios != nil {
		fields = append(fields, negocios.ValidateSchedules(*req.Horarios)...)
		add("horarios", *req.Horarios)
		add("horario_min", negocios.DeriveScheduleMin(*req.Horarios))
	}
	if req.Accesibilidad != nil {
		add("accesibilidad", *req.Accesibilidad)
	}
	if req.Telefono != nil {
		tel := strings.TrimSpace(*req.Telefono)
		if tel != "" && !telefonoValido.MatchString(tel) {
			fields = append(fields, apperr.FieldError{Field: "telefono", Message: "teléfono no válido"})
		}
		add("telefono", tel)
	}
	return set, fields
}

// perfilOferta son precios, servicios, dietas y alérgenos; lo que no viene conserva lo actual
// para validar el conjunto.
func perfilOferta(b negocios.Business, req PerfilRequest) (bson.D, []apperr.FieldError) {
	set := bson.D{}
	var fields []apperr.FieldError
	if req.NivelPrecio != nil || req.PrecioPromedio != nil {
		nivel, promedio := b.PriceLevel, b.AveragePrice
		if req.NivelPrecio != nil {
			nivel = *req.NivelPrecio
		}
		if req.PrecioPromedio != nil {
			promedio = *req.PrecioPromedio
		}
		fields = append(fields, negocios.ValidatePricing(nivel, promedio)...)
		set = append(set, bson.E{Key: "nivel_precio", Value: nivel}, bson.E{Key: "precio_promedio", Value: promedio})
	}
	if req.Servicios != nil || req.OpcionesDieta != nil || req.Alergenos != nil {
		serv, dieta, alerg := elegir(req.Servicios, b.Services), elegir(req.OpcionesDieta, b.DietOptions), elegir(req.Alergenos, b.Allergens)
		fields = append(fields, negocios.ValidateOffer(serv, dieta, alerg)...)
		set = append(set, bson.E{Key: "servicios", Value: serv}, bson.E{Key: "opciones_dieta", Value: dieta}, bson.E{Key: "alergenos", Value: alerg})
	}
	return set, fields
}

func elegir(nuevo *[]string, actual []string) []string {
	if nuevo != nil {
		return nonNil(*nuevo)
	}
	return nonNil(actual)
}

// pendientes junta nombre, dirección y ubicación nuevos con los que ya esperaban revisión.
func pendientes(b negocios.Business, req PerfilRequest, now time.Time) (*negocios.CambiosPendientes, []apperr.FieldError) {
	if req.Nombre == nil && req.Direccion == nil && req.Ubicacion == nil {
		return nil, nil
	}
	c := negocios.CambiosPendientes{}
	if b.CambiosPendientes != nil {
		c = *b.CambiosPendientes
	}
	fields := sumarPendientes(&c, b.Name, req)
	if len(c.Campos) == 0 {
		return nil, fields
	}
	c.EnviadoEn = now
	return &c, fields
}

func sumarPendientes(c *negocios.CambiosPendientes, nombre string, req PerfilRequest) []apperr.FieldError {
	var fields []apperr.FieldError
	marcar := func(campo string) {
		if !slices.Contains(c.Campos, campo) {
			c.Campos = append(c.Campos, campo)
		}
	}
	if req.Nombre != nil && strings.TrimSpace(*req.Nombre) != nombre {
		c.Nombre = strings.TrimSpace(*req.Nombre)
		marcar("nombre")
	}
	if req.Direccion != nil {
		fields = append(fields, negocios.ValidateAddress(*req.Direccion)...)
		d := *req.Direccion
		c.Direccion = &d
		marcar("direccion")
	}
	if req.Ubicacion != nil {
		loc := negocios.Location{Type: req.Ubicacion.Type, Coordinates: req.Ubicacion.Coordinates}
		fields = append(fields, negocios.ValidateLocation(loc)...)
		c.Ubicacion = &loc
		marcar("ubicacion")
	}
	return fields
}

// Pausa pausa (solo desde `aprobado`) o reanuda (solo desde `pausado`). Pausado no sale en nada
// público; las reservas hechas no se cancelan.
func (s *Service) Pausa(ctx context.Context, claims *middleware.Claims, id string, req PausaRequest) (PerfilResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return PerfilResponse{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	var nb negocios.Business
	evento := negocios.AvisoPausado
	if *req.Pausado {
		if req.Hasta != nil && (!req.Hasta.After(now) || req.Hasta.Sub(now) > MaxPausa) {
			return PerfilResponse{}, invalido("hasta", "debe ser futura y de 90 días o menos")
		}
		pausa := negocios.Pausa{Desde: &now, Hasta: req.Hasta, Motivo: strings.TrimSpace(req.Motivo)}
		nb, err = s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "estado", Value: negocios.StatusApproved}, noEliminado()},
			bson.D{{Key: "estado", Value: negocios.StatusPaused}, {Key: "pausado", Value: pausa}})
	} else {
		evento = negocios.AvisoReanudado
		nb, err = s.reanudar(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "estado", Value: negocios.StatusPaused}, noEliminado()})
	}
	if errors.Is(err, negocios.ErrNoAplica) {
		return PerfilResponse{}, errEstado()
	}
	if err != nil {
		return PerfilResponse{}, errStore()
	}
	s.despues(ctx, nb.ID, evento)
	return perfilResponse(nb), nil
}

func (s *Service) reanudar(ctx context.Context, filter bson.D) (negocios.Business, error) {
	return s.repo.ChangeIf(ctx, filter, bson.D{{Key: "estado", Value: negocios.StatusApproved}}, "pausado")
}

// ReservasConfig guarda «Horas y cupo» y copia `acepta_reservas` al campo que filtra /lugares.
// Un anticipo mayor a 0 exige cobros activos (nv-60).
func (s *Service) ReservasConfig(ctx context.Context, claims *middleware.Claims, id string, version int, req ReservasConfigRequest) (ReservasConfigResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return ReservasConfigResponse{}, err
	}
	if !slices.Contains(publicados, b.State) {
		return ReservasConfigResponse{}, errEstado()
	}
	cfg, err := reservasConfig(req, b.Cobros)
	if err != nil {
		return ReservasConfigResponse{}, err
	}
	nb, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "version", Value: version},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: publicados}}}, noEliminado()},
		bson.D{{Key: "reservas_config", Value: cfg}, {Key: "acepta_reservas", Value: cfg.AceptaReservas}})
	if errors.Is(err, negocios.ErrNoAplica) {
		return ReservasConfigResponse{}, apperr.Conflict("el negocio cambió; vuelve a cargarlo")
	}
	if err != nil {
		return ReservasConfigResponse{}, errStore()
	}
	s.despues(ctx, nb.ID, "")
	return ReservasConfigResponse{ReservasConfig: cfg, Version: nb.Version}, nil
}

// reservasConfig valida las combinaciones que el validador no ve.
func reservasConfig(req ReservasConfigRequest, cobros *negocios.Cobros) (negocios.ReservasConfig, error) {
	if req.DuracionTurnoMin%15 != 0 {
		return negocios.ReservasConfig{}, invalido("duracion_turno_min", "debe ser múltiplo de 15")
	}
	if req.Anticipo.Tipo == "porcentaje" && req.Anticipo.Valor > 100 {
		return negocios.ReservasConfig{}, invalido("anticipo.valor", "el porcentaje va de 0 a 100")
	}
	if req.Anticipo.Valor > 0 && (cobros == nil || cobros.Estado != negocios.CobrosActivo) {
		return negocios.ReservasConfig{}, apperr.Validation("cobros_no_activos", apperr.FieldError{Field: "anticipo", Message: "cobros_no_activos"})
	}
	return negocios.ReservasConfig{
		AceptaReservas: *req.AceptaReservas, CapacidadPorHorario: req.CapacidadPorHorario, DuracionTurnoMin: req.DuracionTurnoMin,
		Anticipo:            negocios.Anticipo{Tipo: req.Anticipo.Tipo, Valor: req.Anticipo.Valor},
		PoliticaCancelacion: req.PoliticaCancelacion, Confirmacion: req.Confirmacion, PermiteCambios: *req.PermiteCambios,
	}, nil
}

// QR es el contenido del QR de mostrador; `qr_checkin` no cambia (nv-20).
func (s *Service) QR(ctx context.Context, claims *middleware.Claims, id string) (QRResponse, error) {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return QRResponse{}, err
	}
	return QRResponse{Contenido: ContenidoQR(b.QRCheckin)}, nil
}

// ContenidoQR es lo que codifica el QR de mostrador.
func ContenidoQR(qr string) string { return esquemaQR + qr }

// Baja da de baja el negocio (borrado lógico) si `confirmacion` es su nombre exacto y no tiene
// reservas futuras. Borra el expediente, avisa a los observadores y pide borrar las imágenes;
// el nombre y el `qr_checkin` quedan para reseñas y reservas pasadas.
func (s *Service) Baja(ctx context.Context, claims *middleware.Claims, id string, req BajaRequest) error {
	b, err := s.propio(ctx, claims, id)
	if err != nil {
		return err
	}
	if !ConfirmaBaja(b.Name, req.Confirmacion) {
		return invalido("confirmacion", "escribe el nombre exacto del negocio")
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	if s.futuras != nil {
		hay, err := s.futuras.HayReservasFuturas(ctx, b.ID, now)
		if err != nil {
			return errStore()
		}
		if hay {
			return apperr.Conflict("reservas_futuras")
		}
	}
	if _, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, noEliminado()},
		bson.D{{Key: "eliminado_en", Value: now}}, "cuenta_deposito"); errors.Is(err, negocios.ErrNoAplica) {
		return errNoEncontrado()
	} else if err != nil {
		return errStore()
	}
	s.despues(ctx, b.ID, negocios.AvisoDadoDeBaja)
	s.limpiarBaja(context.WithoutCancel(ctx), b)
	return nil
}

// ConfirmaBaja compara la confirmación con el nombre exacto (sin espacios de los extremos).
func ConfirmaBaja(nombre, confirmacion string) bool {
	return strings.TrimSpace(confirmacion) == nombre
}

// limpiarBaja cancela el expediente y borra las imágenes. Nada de esto cambia la respuesta:
// lo que falle queda en el log (el expediente lo vuelve a intentar ARCO al borrar la cuenta).
func (s *Service) limpiarBaja(ctx context.Context, b negocios.Business) {
	if s.exp != nil {
		if err := s.exp.CancelarNegocio(ctx, b.ID); err != nil {
			s.log.ErrorContext(ctx, "panel: expediente sin cancelar tras la baja", "evento", "expediente_sin_cancelar")
		}
	}
	if s.imagenes == nil {
		return
	}
	for _, img := range s.imagenesDe(ctx, b) {
		if err := s.imagenes.Eliminar(ctx, b.OwnerID.Hex(), img); err != nil {
			s.log.WarnContext(ctx, "panel: imagen sin borrar tras la baja", "evento", "imagen_huerfana")
		}
	}
}

// imagenesDe son las imágenes de nv-45 de la galería y del menú.
func (s *Service) imagenesDe(ctx context.Context, b negocios.Business) []string {
	ids := []string{}
	for _, f := range b.Photos {
		if f.ImagenID != "" {
			ids = append(ids, f.ImagenID)
		}
	}
	items, err := s.menu.DeNegocio(ctx, b.ID)
	if err != nil {
		return ids
	}
	for _, it := range items {
		if it.ImagenID != "" {
			ids = append(ids, it.ImagenID)
		}
	}
	return ids
}

// Sello otorga o retira el Sello de Excelencia (administrador, con motivo); invalida la ficha y
// avisa al dueño. Otorgarlo exige un negocio publicado.
func (s *Service) Sello(ctx context.Context, claims *middleware.Claims, id string, req SelloRequest) (SelloResponse, error) {
	admin, err := sujeto(claims)
	if err != nil {
		return SelloResponse{}, err
	}
	b, err := s.buscar(ctx, id)
	if err != nil {
		return SelloResponse{}, err
	}
	otorgar := *req.Otorgar
	if otorgar && !slices.Contains(publicados, b.State) {
		return SelloResponse{}, invalido("otorgar", "solo un negocio aprobado recibe el Sello")
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	motivo := strings.TrimSpace(req.Motivo)
	nb, err := s.repo.ChangeIf(ctx, bson.D{{Key: "_id", Value: b.ID}, noEliminado()}, bson.D{
		{Key: "sello_excelencia", Value: otorgar},
		{Key: "sello", Value: negocios.Sello{Otorgado: otorgar, OtorgadoPor: admin, OtorgadoEn: now, Motivo: motivo}},
	})
	if errors.Is(err, negocios.ErrNoAplica) {
		return SelloResponse{}, errNoEncontrado()
	}
	if err != nil {
		return SelloResponse{}, errStore()
	}
	s.despues(ctx, nb.ID, "")
	resultado := "sello_retirado"
	if otorgar {
		resultado = "sello_otorgado"
	}
	s.notifier.Emit(ctx, nb.OwnerID, alta.KindRevisado, map[string]string{"negocio_id": nb.ID.Hex(), "resultado": resultado})
	return SelloResponse{NegocioID: nb.ID.Hex(), SelloExcelencia: otorgar, Motivo: motivo, DecididoEn: now, Version: nb.Version}, nil
}
