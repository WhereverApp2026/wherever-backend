// Package configuracion publica las versiones vigentes y el modo mantenimiento (spec nv-16).
// No tiene repositorio: los valores salen de la configuración del proceso.
package configuracion

// Respuesta es el cuerpo de GET /api/v1/configuracion.
type Respuesta struct {
	VersionMinimaApp       string `json:"version_minima_app"`
	VersionActualApp       string `json:"version_actual_app"`
	VersionTerminos        string `json:"version_terminos"`
	VersionAvisoPrivacidad string `json:"version_aviso_privacidad"`
	Mantenimiento          bool   `json:"mantenimiento"`
}
