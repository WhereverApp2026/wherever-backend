package panel

import (
	"context"
	"slices"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Datos del negocio de la maqueta (spec nv-90) en el perfil: con el negocio publicado van a
// `cambios_pendientes`, como nombre, dirección y ubicación.

// Nombres de los campos en `cambios_pendientes.campos`.
const (
	CampoExperiencias = "experiencias"
	CampoAnioApertura = "anio_apertura"
	CampoWhatsApp     = "whatsapp"
	CampoRedes        = "redes"
)

// WithTexto conecta la revisión de palabras bloqueadas de Instagram y el sitio (nv-43).
func (s *Service) WithTexto(t negocios.TextoProhibido) *Service {
	s.texto = t
	return s
}

// traeDatos dice si la petición trae alguno de los datos de nv-90.
func traeDatos(req PerfilRequest) bool {
	return req.Experiencias != nil || req.AnioApertura != nil || req.WhatsApp != nil || req.Redes != nil
}

// sumarDatos valida y agrega los datos de nv-90 a los cambios pendientes.
func sumarDatos(c *negocios.CambiosPendientes, req PerfilRequest, now time.Time, marcar func(string)) []apperr.FieldError {
	var fields []apperr.FieldError
	if req.Experiencias != nil {
		fields = append(fields, negocios.ValidateExperiencias(*req.Experiencias)...)
		c.Experiencias = slices.Clone(*req.Experiencias)
		marcar(CampoExperiencias)
	}
	if req.AnioApertura != nil {
		fields = append(fields, negocios.ValidateAnioApertura(*req.AnioApertura, now)...)
		c.AnioApertura = *req.AnioApertura
		marcar(CampoAnioApertura)
	}
	if req.WhatsApp != nil {
		numero := negocios.NormalizeWhatsApp(*req.WhatsApp)
		fields = append(fields, negocios.ValidateWhatsApp(numero)...)
		c.WhatsApp = &numero
		marcar(CampoWhatsApp)
	}
	if req.Redes != nil {
		r := negocios.NormalizeRedes(*req.Redes)
		fields = append(fields, negocios.ValidateRedes(r)...)
		c.Redes = &r
		marcar(CampoRedes)
	}
	return fields
}

// revisarRedes pasa las redes nuevas por las palabras bloqueadas.
func (s *Service) revisarRedes(ctx context.Context, req PerfilRequest) error {
	if req.Redes == nil {
		return nil
	}
	return negocios.RevisarRedes(ctx, s.texto, negocios.NormalizeRedes(*req.Redes))
}
