package negocios

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Datos del negocio que pide la maqueta (spec nv-90): tipo de experiencia, año de apertura,
// WhatsApp y una red o sitio web. Son públicos por decisión del dueño.

// Catálogo de `experiencias` (pantalla `43_NegRegistro`).
const (
	ExperienciaDesayuno  = "desayuno"
	ExperienciaComida    = "comida"
	ExperienciaCena      = "cena"
	ExperienciaPaseo     = "paseo"
	ExperienciaRomantico = "romantico"
	ExperienciaFamilia   = "familia"
	ExperienciaAmigos    = "amigos"
)

// Experiencias es el catálogo en el orden de la maqueta.
var Experiencias = []string{ExperienciaDesayuno, ExperienciaComida, ExperienciaCena, ExperienciaPaseo,
	ExperienciaRomantico, ExperienciaFamilia, ExperienciaAmigos}

// Límites de los datos de nv-90.
const (
	MaxExperiencias  = 4
	MinAnioApertura  = 1900
	MaxSitioWeb      = 200
	prefijoWhatsApp  = "+52"
	enlaceWhatsAppMX = "https://wa.me/"
)

// Redes es una cuenta de Instagram (sin @) y un sitio web; las dos son opcionales.
type Redes struct {
	Instagram string `bson:"instagram,omitempty" json:"instagram,omitempty"`
	SitioWeb  string `bson:"sitio_web,omitempty" json:"sitio_web,omitempty"`
}

// Vacia dice si no trae nada.
func (r Redes) Vacia() bool { return r.Instagram == "" && r.SitioWeb == "" }

// TextoProhibido revisa un texto contra las palabras bloqueadas de moderación (nv-43). Lo
// cumple moderacion.Service; sin él no se revisan palabras, solo dominios.
type TextoProhibido interface {
	Prohibido(ctx context.Context, texto string) (bool, error)
}

var (
	whatsappValido  = regexp.MustCompile(`^\+52[0-9]{10}$`)
	instagramValido = regexp.MustCompile(`^[A-Za-z0-9._]{1,30}$`)
)

// dominiosNoPermitidos son acortadores conocidos: esconden el destino real.
var dominiosNoPermitidos = []string{
	"bit.ly", "tinyurl.com", "t.co", "goo.gl", "ow.ly", "is.gd", "buff.ly", "cutt.ly",
	"rebrand.ly", "shorturl.at", "tiny.cc", "rb.gy", "t.ly", "s.id", "v.gd",
}

// ValidExperiencia dice si el id está en el catálogo.
func ValidExperiencia(id string) bool { return slices.Contains(Experiencias, id) }

// ValidateExperiencias exige de 1 a 4 del catálogo, sin repetir.
func ValidateExperiencias(ids []string) []apperr.FieldError {
	bad := []apperr.FieldError{{Field: "experiencias", Message: "de 1 a 4 de: desayuno, comida, cena, paseo, romantico, familia, amigos; sin repetir"}}
	if len(ids) == 0 || len(ids) > MaxExperiencias {
		return bad
	}
	for i, id := range ids {
		if !ValidExperiencia(id) || slices.Contains(ids[:i], id) {
			return bad
		}
	}
	return nil
}

// ValidateAnioApertura exige de 1900 al año en curso (zona de los horarios).
func ValidateAnioApertura(anio int, now time.Time) []apperr.FieldError {
	if anio < MinAnioApertura || anio > now.In(MexicoCity()).Year() {
		return []apperr.FieldError{{Field: "anio_apertura", Message: "de 1900 al año actual"}}
	}
	return nil
}

// NormalizeWhatsApp quita espacios, guiones, puntos y paréntesis (como el teléfono de nv-35).
func NormalizeWhatsApp(raw string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-.()", r) {
			return -1
		}
		return r
	}, raw)
}

// ValidateWhatsApp exige E.164 de México: `+52` y 10 dígitos. Vacío = sin WhatsApp.
func ValidateWhatsApp(numero string) []apperr.FieldError {
	if numero != "" && !whatsappValido.MatchString(numero) {
		return []apperr.FieldError{{Field: "whatsapp", Message: "número de México con lada: +52 y 10 dígitos"}}
	}
	return nil
}

// WhatsAppURL es el enlace público al chat; el número no sale en otro campo.
func WhatsAppURL(numero string) string {
	if numero == "" {
		return ""
	}
	return enlaceWhatsAppMX + strings.TrimPrefix(numero, "+")
}

// NormalizeRedes recorta espacios.
func NormalizeRedes(r Redes) Redes {
	return Redes{Instagram: strings.TrimSpace(r.Instagram), SitioWeb: strings.TrimSpace(r.SitioWeb)}
}

// ValidateRedes revisa la forma: Instagram sin @ ni espacios; sitio `https`, hasta 200, sin
// usuario en la URL ni acortadores.
func ValidateRedes(r Redes) []apperr.FieldError {
	var fields []apperr.FieldError
	if r.Instagram != "" && !instagramValido.MatchString(r.Instagram) {
		fields = append(fields, apperr.FieldError{Field: "redes.instagram", Message: "usuario de 1 a 30 letras, números, punto o guion bajo, sin @"})
	}
	if r.SitioWeb != "" && !sitioValido(r.SitioWeb) {
		fields = append(fields, apperr.FieldError{Field: "redes.sitio_web", Message: "dirección https de hasta 200 caracteres, sin acortadores"})
	}
	return fields
}

func sitioValido(raw string) bool {
	if len(raw) > MaxSitioWeb {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, ".") {
		return false
	}
	for _, d := range dominiosNoPermitidos {
		if host == d || strings.HasSuffix(host, "."+d) {
			return false
		}
	}
	return true
}

// RevisarRedes pasa Instagram y el sitio por las palabras bloqueadas. Sin revisor no revisa;
// si el revisor falla, 503 (no se publica un texto sin revisar).
func RevisarRedes(ctx context.Context, t TextoProhibido, r Redes) error {
	if t == nil {
		return nil
	}
	for _, par := range [][2]string{{"redes.instagram", r.Instagram}, {"redes.sitio_web", r.SitioWeb}} {
		campo, texto := par[0], par[1]
		if texto == "" {
			continue
		}
		malo, err := t.Prohibido(ctx, texto)
		if err != nil {
			return apperr.Unavailable("servicio no disponible")
		}
		if malo {
			return apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: "contiene palabras no permitidas"})
		}
	}
	return nil
}

// ExperienciaDeHora es la comida que toca a esa hora en México: desayuno (6–11), comida
// (12–17) o cena (18–23). De madrugada, ninguna.
func ExperienciaDeHora(t time.Time) string {
	switch h := t.In(MexicoCity()).Hour(); {
	case h >= 6 && h < 12:
		return ExperienciaDesayuno
	case h >= 12 && h < 18:
		return ExperienciaComida
	case h >= 18:
		return ExperienciaCena
	default:
		return ""
	}
}

// ExperienciaDeCompania lleva la compañía del asistente (nv-31) a una experiencia: pareja →
// romántico, familia y amigos igual. Solo, o sin dato, ninguna.
func ExperienciaDeCompania(compania string) string {
	switch compania {
	case "pareja":
		return ExperienciaRomantico
	case ExperienciaFamilia, ExperienciaAmigos:
		return compania
	default:
		return ""
	}
}
