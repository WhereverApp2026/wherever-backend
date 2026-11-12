package config

import (
	"fmt"
	"strings"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/versionado"
)

// Defectos de nv-16. `TERMINOS_VERSION` es la que hoy manda la app al registrarse; `AVISO_VERSION`
// sube cuando cambia docs/aviso-privacidad.md.
const (
	DefaultAppVersionMinima = "0.0.0"
	DefaultAppVersionActual = "1.0.0"
	DefaultTerminosVersion  = "2026-10"
	DefaultAvisoVersion     = "2026-11"
	maxDocVersionLen        = 20
)

// VersionApp son las versiones públicas y el modo mantenimiento (nv-16). Se leen al arrancar y
// no cambian en caliente.
type VersionApp struct {
	Minima        versionado.Version
	Actual        versionado.Version
	Terminos      string
	Aviso         string
	Mantenimiento bool
}

func (v *VersionApp) parse(getenv func(string) string) []error {
	var errs []error
	var err error
	if v.Minima, err = versionado.Parse(orDefault(strings.TrimSpace(getenv(KeyAppVersionMinima)), DefaultAppVersionMinima)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", KeyAppVersionMinima, err))
	}
	if v.Actual, err = versionado.Parse(orDefault(strings.TrimSpace(getenv(KeyAppVersionActual)), DefaultAppVersionActual)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", KeyAppVersionActual, err))
	}
	if len(errs) == 0 && v.Actual.MenorQue(v.Minima) {
		errs = append(errs, fmt.Errorf("%s no puede ser mayor que %s", KeyAppVersionMinima, KeyAppVersionActual))
	}
	v.Terminos = orDefault(strings.TrimSpace(getenv(KeyTerminosVersion)), DefaultTerminosVersion)
	v.Aviso = orDefault(strings.TrimSpace(getenv(KeyAvisoVersion)), DefaultAvisoVersion)
	errs = append(errs, validateDocVersion(KeyTerminosVersion, v.Terminos)...)
	errs = append(errs, validateDocVersion(KeyAvisoVersion, v.Aviso)...)
	switch strings.ToLower(strings.TrimSpace(getenv(KeyMantenimiento))) {
	case "", "false":
	case "true":
		v.Mantenimiento = true
	default:
		errs = append(errs, fmt.Errorf("%s admite solo true o false", KeyMantenimiento))
	}
	return errs
}

func validateDocVersion(key, value string) []error {
	if len(value) > maxDocVersionLen || strings.ContainsAny(value, " \t") {
		return []error{fmt.Errorf("%s debe tener de 1 a %d caracteres sin espacios", key, maxDocVersionLen)}
	}
	return nil
}
