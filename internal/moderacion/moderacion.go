// Package moderacion clasifica imágenes y texto antes de publicarlos (spec nv-42). Los
// llamadores dependen solo de Moderador.Evaluar; los proveedores van detrás de ImagenModerador
// (Amazon Rekognition) y TextoModerador (el modelo de nv-26). Cierra ante fallos: si algo
// falla, el contenido queda en cuarentena y nunca se aprueba.
package moderacion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
)

// Veredicto es lo que decide si el contenido se publica.
type Veredicto string

// Solo se publica con VeredictoAprobado.
const (
	VeredictoAprobado   Veredicto = "aprobado"
	VeredictoCuarentena Veredicto = "cuarentena"
	VeredictoRechazado  Veredicto = "rechazado"
)

// Filtros que devuelven los adaptadores. Un puntaje va de 0 a 100.
const (
	FiltroDesnudez        = "desnudez"
	FiltroViolencia       = "violencia"
	FiltroDrogas          = "drogas"
	FiltroOdio            = "odio"
	FiltroSexual          = "sexual"
	FiltroAcoso           = "acoso"
	FiltroSpam            = "spam"
	FiltroPalabras        = "palabra_bloqueada"
	FiltroDatosPersonales = "datos_personales"
)

// Fallas que quedan en Resultado.Falla para la cola de revisión (nv-43).
const (
	FallaVacio       = "contenido_vacio"
	FallaReglas      = "reglas"
	FallaImagen      = "imagen"
	FallaTexto       = "texto"
	FallaSinServicio = "sin_servicio"
)

// Errores de los adaptadores. Cualquiera de ellos deja el contenido en cuarentena.
var (
	ErrProveedor     = errors.New("moderacion: error del proveedor")
	ErrImagenGrande  = errors.New("moderacion: imagen mayor al límite del proveedor")
	ErrTextoLargo    = errors.New("moderacion: texto mayor al límite")
	ErrReglasInvalid = errors.New("moderacion: reglas inválidas")
)

// Contenido es lo que se evalúa: texto, imagen o los dos. Nunca lleva datos del autor.
type Contenido struct {
	Texto  string
	Imagen []byte
}

// Resultado es el veredicto con los puntajes por filtro. Falla dice qué parte no se pudo
// evaluar; si no está vacía, el veredicto nunca es aprobado.
type Resultado struct {
	Veredicto Veredicto
	Puntajes  map[string]float64
	Falla     string
}

// Publicable dice si el contenido se puede publicar.
func (r Resultado) Publicable() bool { return r.Veredicto == VeredictoAprobado }

// ImagenModerador devuelve los puntajes de una imagen por filtro.
type ImagenModerador interface {
	EvaluarImagen(ctx context.Context, imagen []byte) (map[string]float64, error)
}

// TextoModerador devuelve los puntajes de un texto por filtro.
type TextoModerador interface {
	EvaluarTexto(ctx context.Context, texto string) (map[string]float64, error)
}

// FuenteReglas entrega las reglas vigentes: Service las lee de moderacion_reglas (nv-43) y
// Fijas las deja constantes.
type FuenteReglas interface {
	Reglas(ctx context.Context) (Reglas, error)
}

// Moderador junta los adaptadores y las reglas.
type Moderador struct {
	imagen ImagenModerador
	texto  TextoModerador
	reglas FuenteReglas
	log    *slog.Logger
}

// New arma el moderador. Un adaptador nil deja en cuarentena todo lo de su tipo.
func New(imagen ImagenModerador, texto TextoModerador, reglas FuenteReglas, log *slog.Logger) *Moderador {
	if log == nil {
		log = slog.Default()
	}
	return &Moderador{imagen: imagen, texto: texto, reglas: reglas, log: log}
}

// Evaluar clasifica el contenido. Nunca devuelve aprobado si una parte falló o si no hay
// nada que evaluar; si una parte ya rechaza, rechaza aunque otra haya fallado.
func (m *Moderador) Evaluar(ctx context.Context, c Contenido) Resultado {
	res := Resultado{Puntajes: map[string]float64{}}
	if c.Texto == "" && len(c.Imagen) == 0 {
		res.Veredicto, res.Falla = VeredictoCuarentena, FallaVacio
		return res
	}
	reglas, err := m.cargarReglas(ctx)
	if err != nil {
		m.log.WarnContext(ctx, "moderación sin reglas", slog.String("parte", "reglas"), slog.String("error", err.Error()))
		res.Veredicto, res.Falla = VeredictoCuarentena, FallaReglas
		return res
	}
	if len(c.Imagen) > 0 {
		m.sumar(ctx, &res, FallaImagen, func() (map[string]float64, error) {
			if m.imagen == nil {
				return nil, errSinServicio
			}
			return m.imagen.EvaluarImagen(ctx, c.Imagen)
		})
	}
	if c.Texto != "" {
		m.sumar(ctx, &res, FallaTexto, func() (map[string]float64, error) {
			return m.evaluarTexto(ctx, reglas, c.Texto)
		})
	}
	res.Veredicto = reglas.Clasificar(maximo(res.Puntajes))
	if res.Falla != "" && res.Veredicto == VeredictoAprobado {
		res.Veredicto = VeredictoCuarentena
	}
	return res
}

var errSinServicio = errors.New("moderacion: adaptador sin configurar")

func (m *Moderador) cargarReglas(ctx context.Context) (Reglas, error) {
	if m.reglas == nil {
		return Reglas{}, errSinServicio
	}
	r, err := m.reglas.Reglas(ctx)
	if err != nil {
		return Reglas{}, err
	}
	return r, r.Validar()
}

// sumar agrega los puntajes de una parte o anota su falla. Solo registra el tipo de error,
// nunca el contenido.
func (m *Moderador) sumar(ctx context.Context, res *Resultado, parte string, evaluar func() (map[string]float64, error)) {
	puntajes, err := evaluar()
	for filtro, p := range puntajes {
		res.Puntajes[filtro] = max(res.Puntajes[filtro], p)
	}
	if err != nil {
		falla := parte
		if errors.Is(err, errSinServicio) {
			falla = FallaSinServicio
		}
		if res.Falla == "" {
			res.Falla = falla
		}
		m.log.WarnContext(ctx, "moderación sin resultado", slog.String("parte", parte), slog.String("error", err.Error()))
	}
}

// evaluarTexto aplica primero las reglas locales; una palabra bloqueada rechaza sin gastar
// una llamada al modelo.
func (m *Moderador) evaluarTexto(ctx context.Context, reglas Reglas, texto string) (map[string]float64, error) {
	if len([]rune(texto)) > MaxRunasTexto {
		return nil, ErrTextoLargo
	}
	locales := reglas.Locales(texto)
	if locales[FiltroPalabras] >= reglas.Rechazo {
		return locales, nil
	}
	if m.texto == nil {
		return locales, errSinServicio
	}
	modelo, err := m.texto.EvaluarTexto(ctx, texto)
	if err != nil {
		return locales, err
	}
	out := maps.Clone(locales)
	for filtro, p := range modelo {
		out[filtro] = max(out[filtro], p)
	}
	return out, nil
}

func maximo(puntajes map[string]float64) float64 {
	var m float64
	for _, p := range puntajes {
		m = max(m, p)
	}
	return m
}

// validarPuntaje revisa que un puntaje del proveedor esté entre 0 y 100.
func validarPuntaje(filtro string, p float64) error {
	if p < 0 || p > 100 {
		return fmt.Errorf("%w: puntaje de %s fuera de 0..100", ErrProveedor, filtro)
	}
	return nil
}
