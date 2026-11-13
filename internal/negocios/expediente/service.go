package expediente

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Almacen es el bucket privado de nv-44 (interfaz externa: S3). Es un subconjunto del
// almacén de imágenes de nv-45, así el mismo adaptador de S3 sirve para los dos.
type Almacen interface {
	Leer(ctx context.Context, clave string, max int64) ([]byte, error)
	Guardar(ctx context.Context, clave, tipo string, datos []byte) error
	Borrar(ctx context.Context, claves ...string) error
}

// Service cifra, guarda y lee el expediente. Sin llavero (`DATA_KEYS`) o sin almacén responde
// 503: nunca guarda en claro.
type Service struct {
	repo    *Repository
	llavero *cifrado.Llavero
	almacen Almacen
	log     *slog.Logger
	now     func() time.Time
}

// NewService recibe todo por constructor; llavero y almacén pueden ser nil (503).
func NewService(repo *Repository, llavero *cifrado.Llavero, almacen Almacen, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, llavero: llavero, almacen: almacen, log: log, now: time.Now}
}

// WithClock cambia el reloj (pruebas de retención).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func errStore() error        { return apperr.Unavailable("servicio no disponible") }
func errSinCifrado() error   { return apperr.Unavailable("el expediente no está disponible") }
func errNoEncontrado() error { return apperr.NotFound("documento no encontrado") }

// Metadato es lo que se puede mostrar de un registro sin descifrarlo.
type Metadato struct {
	Tipo        string     `json:"tipo"`
	ContentType string     `json:"content_type,omitempty"`
	Tamano      int64      `json:"tamano"`
	SubidoEn    time.Time  `json:"subido_en"`
	Estado      string     `json:"estado"`
	BorrarEn    *time.Time `json:"borrar_despues_de,omitempty"`
}

func metadato(d Documento) Metadato {
	return Metadato{Tipo: d.Tipo, ContentType: d.ContentType, Tamano: d.Tamano, SubidoEn: d.SubidoEn.UTC(), Estado: d.Estado, BorrarEn: d.BorrarDespuesDe}
}

// Contenido es un documento descifrado listo para entregarse.
type Contenido struct {
	ContentType string
	Nombre      string
	Datos       []byte
}

func lugar(negocio bson.ObjectID, tipo string) cifrado.Lugar {
	return cifrado.Lugar{Coleccion: Collection, Campo: tipo, ID: negocio.Hex()}
}

// SubirArchivo valida el tipo real, quita metadatos, cifra y sube a `privado/`. Reemplaza el
// anterior del mismo tipo y borra su objeto.
func (s *Service) SubirArchivo(ctx context.Context, negocio, owner bson.ObjectID, tipo string, datos []byte) (Metadato, error) {
	if s.llavero == nil || s.almacen == nil {
		return Metadato{}, errSinCifrado()
	}
	contentType, limpio, err := prepararArchivo(tipo, datos)
	if err != nil {
		return Metadato{}, err
	}
	clave, err := s.sellarYSubir(ctx, negocio, tipo, limpio)
	if err != nil {
		return Metadato{}, err
	}
	d := Documento{NegocioID: negocio, PropietarioID: owner, Tipo: tipo, LlaveS3: clave, ContentType: contentType,
		Tamano: int64(len(limpio)), SubidoEn: s.now().UTC().Truncate(time.Millisecond), Estado: EstadoVigente}
	antes, err := s.repo.Reemplazar(ctx, d)
	if err != nil {
		s.borrarObjetos(ctx, clave) // sin registro, el objeto queda huérfano: se quita
		return Metadato{}, s.storeErr(err)
	}
	if antes != nil && antes.LlaveS3 != "" {
		s.borrarObjetos(ctx, antes.LlaveS3)
	}
	return metadato(d), nil
}

// prepararArchivo revisa el tipo de documento y el archivo. ErrGrande va tal cual: el handler
// responde 413.
func prepararArchivo(tipo string, datos []byte) (string, []byte, error) {
	if !slices.Contains(TiposArchivo, tipo) {
		return "", nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "tipo", Message: "debe ser ine_frente, ine_reverso o comprobante_domicilio"})
	}
	contentType, limpio, err := Preparar(datos)
	if errors.Is(err, ErrGrande) {
		return "", nil, ErrGrande
	}
	if err != nil {
		return "", nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: "debe ser JPEG, PNG o PDF"})
	}
	return contentType, limpio, nil
}

// sellarYSubir cifra los bytes atados a (negocio, tipo) y los sube a `privado/`.
func (s *Service) sellarYSubir(ctx context.Context, negocio bson.ObjectID, tipo string, limpio []byte) (string, error) {
	sellado, err := s.llavero.Cifrar(lugar(negocio, tipo), string(limpio))
	if err != nil {
		return "", errStore()
	}
	sufijo, err := aleatorio()
	if err != nil {
		return "", errStore()
	}
	clave := prefijoPrivado + negocio.Hex() + "/" + tipo + "/" + sufijo + extensionCifrada
	if err := s.almacen.Guardar(ctx, clave, "application/octet-stream", []byte(sellado)); err != nil {
		return "", errStore()
	}
	return clave, nil
}

func (s *Service) storeErr(err error) error {
	if errors.Is(err, ErrConcurrente) {
		return apperr.Conflict("el documento cambió; intenta de nuevo")
	}
	return errStore()
}

// GuardarFiscales valida y guarda los datos fiscales cifrados.
func (s *Service) GuardarFiscales(ctx context.Context, negocio, owner bson.ObjectID, f DatosFiscales) (Metadato, error) {
	if s.llavero == nil {
		return Metadato{}, errSinCifrado()
	}
	f.RFC = NormalizarRFC(f.RFC)
	var fields []apperr.FieldError
	if !RFCValido(f.RFC) {
		fields = append(fields, apperr.FieldError{Field: "rfc", Message: "rfc_invalido"})
	}
	if !RegimenValido(f.RegimenFiscal) {
		fields = append(fields, apperr.FieldError{Field: "regimen_fiscal", Message: "no está en el catálogo del SAT"})
	}
	if !UsoCFDIValido(f.UsoCFDI) {
		fields = append(fields, apperr.FieldError{Field: "uso_cfdi", Message: "no está en el catálogo del SAT"})
	}
	if !CPValido(f.CPFiscal) {
		fields = append(fields, apperr.FieldError{Field: "cp_fiscal", Message: "debe tener 5 dígitos"})
	}
	if len(fields) > 0 {
		return Metadato{}, apperr.Validation("datos inválidos", fields...)
	}
	return s.guardarJSON(ctx, negocio, owner, TipoFiscales, f)
}

// GuardarCuenta valida la CLABE (banco y dígito verificador), la guarda cifrada y devuelve la
// cuenta para enmascararla en `negocios.cuenta_deposito`.
func (s *Service) GuardarCuenta(ctx context.Context, negocio, owner bson.ObjectID, clabe, titular string) (Cuenta, error) {
	if s.llavero == nil {
		return Cuenta{}, errSinCifrado()
	}
	banco, ok := BancoCLABE(clabe)
	if !ok {
		return Cuenta{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "clabe", Message: "clabe_invalida"})
	}
	c := Cuenta{CLABE: clabe, Banco: banco, Titular: titular}
	if _, err := s.guardarJSON(ctx, negocio, owner, TipoCuenta, c); err != nil {
		return Cuenta{}, err
	}
	return c, nil
}

func (s *Service) guardarJSON(ctx context.Context, negocio, owner bson.ObjectID, tipo string, v any) (Metadato, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return Metadato{}, errStore()
	}
	sellado, err := s.llavero.Cifrar(lugar(negocio, tipo), string(raw))
	if err != nil {
		return Metadato{}, errStore()
	}
	d := Documento{NegocioID: negocio, PropietarioID: owner, Tipo: tipo, ContentType: "application/json",
		Tamano: int64(len(raw)), DatosCifrados: sellado, SubidoEn: s.now().UTC().Truncate(time.Millisecond), Estado: EstadoVigente}
	if _, err := s.repo.Reemplazar(ctx, d); err != nil {
		return Metadato{}, s.storeErr(err)
	}
	return metadato(d), nil
}

// Listar devuelve los metadatos del expediente del negocio (sin descifrar nada).
func (s *Service) Listar(ctx context.Context, negocio bson.ObjectID) ([]Metadato, error) {
	rows, err := s.repo.Listar(ctx, negocio)
	if err != nil {
		return nil, errStore()
	}
	out := make([]Metadato, 0, len(rows))
	for _, d := range rows {
		out = append(out, metadato(d))
	}
	return out, nil
}

// Vigentes dice qué tipos están vigentes (para el paso 6 y para enviar a revisión).
func (s *Service) Vigentes(ctx context.Context, negocio bson.ObjectID) (map[string]bool, error) {
	rows, err := s.repo.Listar(ctx, negocio)
	if err != nil {
		return nil, errStore()
	}
	out := map[string]bool{}
	for _, d := range rows {
		if d.Estado == EstadoVigente {
			out[d.Tipo] = true
		}
	}
	return out, nil
}

// LeerDueno descifra un documento vigente para su dueño (sin bitácora). Los datos fiscales
// bloqueados tras la baja ya no los lee el dueño.
func (s *Service) LeerDueno(ctx context.Context, negocio bson.ObjectID, tipo string) (Contenido, error) {
	return s.leer(ctx, negocio, tipo, false)
}

// LeerAdmin registra la lectura en `accesos_expediente` y solo después descifra. Si la
// bitácora falla, no entrega nada (fail-closed).
func (s *Service) LeerAdmin(ctx context.Context, a Acceso) (Contenido, error) {
	if s.llavero == nil {
		return Contenido{}, errSinCifrado()
	}
	if !slices.Contains(TiposLectura, a.Tipo) {
		return Contenido{}, errNoEncontrado()
	}
	if _, err := s.repo.Buscar(ctx, a.NegocioID, a.Tipo); err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			return Contenido{}, errNoEncontrado()
		}
		return Contenido{}, errStore()
	}
	a.LeidoEn = s.now().UTC()
	if err := s.repo.RegistrarAcceso(ctx, a); err != nil {
		s.log.ErrorContext(ctx, "expediente: bitácora sin escribir; no se entrega el documento", "evento", "bitacora_fallida")
		return Contenido{}, errStore()
	}
	return s.leer(ctx, a.NegocioID, a.Tipo, true)
}

func (s *Service) leer(ctx context.Context, negocio bson.ObjectID, tipo string, admin bool) (Contenido, error) {
	if s.llavero == nil {
		return Contenido{}, errSinCifrado()
	}
	if !slices.Contains(TiposLectura, tipo) {
		return Contenido{}, errNoEncontrado()
	}
	d, err := s.repo.Buscar(ctx, negocio, tipo)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && d.Estado == EstadoBloqueado && !admin) {
		return Contenido{}, errNoEncontrado()
	}
	if err != nil {
		return Contenido{}, errStore()
	}
	return s.abrir(ctx, d)
}

// abrir descifra el JSON guardado o baja y descifra el objeto de S3.
func (s *Service) abrir(ctx context.Context, d Documento) (Contenido, error) {
	negocio, tipo := d.NegocioID, d.Tipo
	if d.DatosCifrados != "" {
		plano, err := s.llavero.Descifrar(lugar(negocio, tipo), d.DatosCifrados)
		if err != nil {
			return Contenido{}, errStore()
		}
		return Contenido{ContentType: "application/json", Nombre: tipo + ".json", Datos: []byte(plano)}, nil
	}
	if s.almacen == nil {
		return Contenido{}, errSinCifrado()
	}
	sellado, err := s.almacen.Leer(ctx, d.LlaveS3, 2*MaxBytes)
	if err != nil {
		return Contenido{}, errStore()
	}
	plano, err := s.llavero.Descifrar(lugar(negocio, tipo), string(sellado))
	if err != nil {
		return Contenido{}, errStore()
	}
	return Contenido{ContentType: d.ContentType, Nombre: tipo + "." + Extension(d.ContentType), Datos: []byte(plano)}, nil
}

// ProgramarBorrado fija el borrado de la INE y del comprobante a `revisado + 90 días`.
func (s *Service) ProgramarBorrado(ctx context.Context, negocio bson.ObjectID, revisado time.Time) error {
	if err := s.repo.ProgramarBorrado(ctx, negocio, TiposArchivo, revisado.Add(RetencionINE)); err != nil {
		return errStore()
	}
	return nil
}

// Cancelar es la cancelación u oposición de ARCO para el expediente de la persona: borra al
// momento INE, comprobante y CLABE (objetos incluidos) y bloquea los datos fiscales hasta
// 5 años después de la baja.
func (s *Service) Cancelar(ctx context.Context, owner bson.ObjectID) error {
	rows, err := s.repo.DePropietario(ctx, owner)
	if err != nil {
		return err
	}
	hasta := s.now().UTC().Add(RetencionFiscales)
	var errs []error
	for _, d := range rows {
		if d.Tipo == TipoFiscales {
			if d.Estado != EstadoBloqueado {
				errs = append(errs, s.repo.Bloquear(ctx, d.ID, hasta))
			}
			continue
		}
		errs = append(errs, s.borrar(ctx, d))
	}
	return errors.Join(errs...)
}

// Retencion borra lo vencido (INE y comprobante a 90 días de la revisión; fiscales a 5 años de
// la baja). Corre en el trabajo nocturno de nv-05, con su candado. Devuelve cuántos borró.
func (s *Service) Retencion(ctx context.Context, limit int64) (int, error) {
	rows, err := s.repo.Vencidos(ctx, s.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, d := range rows {
		if err := s.borrar(ctx, d); err != nil {
			errs = append(errs, err)
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// borrar quita el objeto de S3 (si hay) y después el registro. Si S3 falla, el registro sigue
// y se reintenta la noche siguiente.
func (s *Service) borrar(ctx context.Context, d Documento) error {
	if d.LlaveS3 != "" {
		if s.almacen == nil {
			return errors.New("expediente: sin almacén para borrar el objeto")
		}
		if err := s.almacen.Borrar(ctx, d.LlaveS3); err != nil {
			return err
		}
	}
	return s.repo.Borrar(ctx, d.ID)
}

func (s *Service) borrarObjetos(ctx context.Context, claves ...string) {
	if err := s.almacen.Borrar(context.WithoutCancel(ctx), claves...); err != nil {
		s.log.WarnContext(ctx, "expediente: objeto anterior sin borrar", "evento", "objeto_huerfano")
	}
}

func aleatorio() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
