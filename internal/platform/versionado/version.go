// Package versionado compara versiones de la app y decide el 426 o el 503 de mantenimiento
// (spec nv-16). No depende de HTTP ni de otros paquetes internos.
package versionado

import (
	"errors"
	"strconv"
	"strings"
)

// Prefijo es el único árbol de rutas donde actúa la política.
const Prefijo = "/api/v1/"

// ErrFormato indica una versión que no es semver numérico X.Y.Z.
var ErrFormato = errors.New("la versión debe tener la forma X.Y.Z con números")

// Version es un semver numérico; se compara por componente, nunca como texto.
type Version struct {
	Mayor, Menor, Parche int
}

// Parse acepta `X.Y.Z` con una `v` inicial opcional. Pre-release, `+build`, componentes
// faltantes o extra son inválidos.
func Parse(raw string) (Version, error) {
	partes := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "v"), ".")
	if len(partes) != 3 {
		return Version{}, ErrFormato
	}
	var nums [3]int
	for i, p := range partes {
		n, err := componente(p)
		if err != nil {
			return Version{}, err
		}
		nums[i] = n
	}
	return Version{Mayor: nums[0], Menor: nums[1], Parche: nums[2]}, nil
}

func componente(p string) (int, error) {
	if p == "" || len(p) > 9 || strings.Trim(p, "0123456789") != "" {
		return 0, ErrFormato
	}
	return strconv.Atoi(p)
}

// String devuelve la forma canónica X.Y.Z.
func (v Version) String() string {
	return strconv.Itoa(v.Mayor) + "." + strconv.Itoa(v.Menor) + "." + strconv.Itoa(v.Parche)
}

// MenorQue compara por componente.
func (v Version) MenorQue(o Version) bool {
	if v.Mayor != o.Mayor {
		return v.Mayor < o.Mayor
	}
	if v.Menor != o.Menor {
		return v.Menor < o.Menor
	}
	return v.Parche < o.Parche
}

// Decision es lo que la etapa del pipeline debe hacer con una petición.
type Decision int

// Resultados de Decidir.
const (
	Seguir Decision = iota
	Actualizar
	Mantenimiento
)

// Politica reúne la versión mínima, el mantenimiento y las rutas exentas.
type Politica struct {
	Minima        Version
	Mantenimiento bool
	exactas       map[string]bool
	prefijos      []string
}

// NuevaPolitica arma la política con las rutas exentas de nv-16. Las de documentación solo
// existen en dev; fuera de `/api/v1/` la política no actúa, así que no hace falta listarlas.
func NuevaPolitica(minima Version, mantenimiento bool) Politica {
	return Politica{
		Minima:        minima,
		Mantenimiento: mantenimiento,
		// Stripe no manda X-App-Version: su webhook (nv-60) nunca recibe 426 ni mantenimiento.
		exactas:  map[string]bool{"/api/v1/ping": true, "/api/v1/configuracion": true, "/api/v1/webhooks/stripe": true},
		prefijos: []string{"/api/v1/auth/"},
	}
}

// Exenta indica si la ruta no se bloquea: fuera de `/api/v1/` sigue al enrutador (404 si no
// existe) y dentro quedan libres ping, configuración y auth.
func (p Politica) Exenta(ruta string) bool {
	if !strings.HasPrefix(ruta, Prefijo) || p.exactas[ruta] {
		return true
	}
	for _, pre := range p.prefijos {
		if strings.HasPrefix(ruta, pre) {
			return true
		}
	}
	return false
}

// Decidir aplica la política. El mantenimiento manda sobre el 426; una cabecera ausente o
// ilegible se ignora.
func (p Politica) Decidir(ruta, cabecera string) Decision {
	if p.Exenta(ruta) {
		return Seguir
	}
	if p.Mantenimiento {
		return Mantenimiento
	}
	v, err := Parse(cabecera)
	if err == nil && v.MenorQue(p.Minima) {
		return Actualizar
	}
	return Seguir
}
