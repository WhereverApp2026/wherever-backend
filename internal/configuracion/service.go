package configuracion

import (
	"encoding/json"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/etag"
)

// Service serializa la configuración una sola vez: el cuerpo y el ETag salen de los mismos
// bytes y no cambian mientras vive el proceso.
type Service struct {
	cuerpo []byte
	etag   string
}

// NewService recibe las versiones por constructor. La respuesta solo tiene textos y un booleano:
// json.Marshal no puede fallar con ella.
func NewService(v config.VersionApp) *Service {
	cuerpo, _ := json.Marshal(Respuesta{
		VersionMinimaApp:       v.Minima.String(),
		VersionActualApp:       v.Actual.String(),
		VersionTerminos:        v.Terminos,
		VersionAvisoPrivacidad: v.Aviso,
		Mantenimiento:          v.Mantenimiento,
	})
	return &Service{cuerpo: cuerpo, etag: etag.De(cuerpo)}
}

// Cuerpo devuelve una copia del JSON para que nadie altere el original.
func (s *Service) Cuerpo() []byte { return append([]byte(nil), s.cuerpo...) }

// ETag devuelve el ETag débil del cuerpo.
func (s *Service) ETag() string { return s.etag }
