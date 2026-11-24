package alta

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Datos del negocio de la maqueta (spec nv-90) en el alta y en la decisión de cambios.

// WithTexto conecta la revisión de palabras bloqueadas de Instagram y el sitio (nv-43).
func (s *Service) WithTexto(t negocios.TextoProhibido) *Service {
	s.texto = t
	return s
}

// datosCrear valida experiencias y año del paso 1 y los pone en el negocio.
func datosCrear(b *negocios.Business, req CrearRequest, now time.Time) []apperr.FieldError {
	var fields []apperr.FieldError
	if req.Experiencias != nil {
		fields = append(fields, negocios.ValidateExperiencias(req.Experiencias)...)
		b.Experiencias = req.Experiencias
	}
	if req.AnioApertura != nil {
		fields = append(fields, negocios.ValidateAnioApertura(*req.AnioApertura, now)...)
		b.AnioApertura = *req.AnioApertura
	}
	return fields
}

// contactoPaso2 son WhatsApp y redes del paso 2: lo que no viene no se toca; vacío lo quita.
func contactoPaso2(p *Paso2Request) (bson.D, []apperr.FieldError) {
	set := bson.D{}
	var fields []apperr.FieldError
	if p.WhatsApp != nil {
		numero := negocios.NormalizeWhatsApp(*p.WhatsApp)
		fields = append(fields, negocios.ValidateWhatsApp(numero)...)
		set = append(set, bson.E{Key: "whatsapp", Value: numero})
	}
	if p.Redes != nil {
		r := negocios.NormalizeRedes(*p.Redes)
		p.Redes = &r
		fields = append(fields, negocios.ValidateRedes(r)...)
		set = append(set, bson.E{Key: "redes", Value: r})
	}
	return set, fields
}

// revisarPaso revisa las redes del paso 2 contra las palabras bloqueadas.
func (s *Service) revisarPaso(ctx context.Context, p Paso) error {
	if p.Numero != 2 || p.Dos == nil || p.Dos.Redes == nil {
		return nil
	}
	return negocios.RevisarRedes(ctx, s.texto, *p.Dos.Redes)
}

// setDatosCambios agrega al `$set` los datos de nv-90 que esperaban revisión.
func setDatosCambios(set bson.D, c *negocios.CambiosPendientes) bson.D {
	if c.Experiencias != nil {
		set = append(set, bson.E{Key: "experiencias", Value: c.Experiencias})
	}
	if c.AnioApertura != 0 {
		set = append(set, bson.E{Key: "anio_apertura", Value: c.AnioApertura})
	}
	if c.WhatsApp != nil {
		set = append(set, bson.E{Key: "whatsapp", Value: *c.WhatsApp})
	}
	if c.Redes != nil {
		set = append(set, bson.E{Key: "redes", Value: *c.Redes})
	}
	return set
}
