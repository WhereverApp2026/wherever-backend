package alta

import (
	"context"
	"errors"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Fotos: la imagen se sube y se modera con nv-45 (`POST /imagenes/subida`); aquí solo se
// registra la ya publicada en la galería, se quita o se ordena.

// AgregarFoto agrega a la galería una imagen publicada de la persona.
func (s *Service) AgregarFoto(ctx context.Context, claims *middleware.Claims, id string, req FotoRequest) (NegocioResponse, error) {
	if s.imagenes == nil {
		return NegocioResponse{}, apperr.Unavailable("la subida de fotos no está disponible")
	}
	owner, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return NegocioResponse{}, err
	}
	if len(b.Photos) >= negocios.MaxPhotos {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "fotos", Message: "máximo 10 fotos"})
	}
	if slices.ContainsFunc(b.Photos, func(f negocios.Foto) bool { return f.ImagenID == req.ImagenID }) {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "imagen_id", Message: "la foto ya está en la galería"})
	}
	url, err := s.imagenes.Publicada(ctx, owner, req.ImagenID)
	if errors.Is(err, ErrImagenNoValida) {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "imagen_id", Message: "la imagen no está publicada"})
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	fotos := append(slices.Clone(b.Photos), negocios.Foto{ImagenID: req.ImagenID, URL: url, Tipo: req.Tipo, Orden: len(b.Photos)})
	return s.guardarFotos(ctx, b, fotos)
}

// QuitarFoto quita una foto de la galería. Un negocio en revisión o aprobado no baja de 3.
func (s *Service) QuitarFoto(ctx context.Context, claims *middleware.Claims, id, fotoID string) error {
	_, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(b.Photos, func(f negocios.Foto) bool { return f.ImagenID != "" && f.ImagenID == fotoID })
	if i < 0 {
		return apperr.NotFound("foto no encontrada")
	}
	if (b.State == negocios.StatusReview || b.State == negocios.StatusApproved) && len(b.Photos)-1 < negocios.MinPhotos {
		return apperr.Validation("datos inválidos", apperr.FieldError{Field: "fotos", Message: "un negocio publicado o en revisión necesita al menos 3 fotos"})
	}
	fotos := slices.Delete(slices.Clone(b.Photos), i, i+1)
	for j := range fotos {
		fotos[j].Orden = j
	}
	_, err = s.guardarFotos(ctx, b, fotos)
	return err
}

// OrdenarFotos cambia el orden de la galería; la primera es la portada.
func (s *Service) OrdenarFotos(ctx context.Context, claims *middleware.Claims, id string, req OrdenRequest) (NegocioResponse, error) {
	_, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return NegocioResponse{}, err
	}
	porID := map[string]negocios.Foto{}
	for _, f := range b.Photos {
		porID[f.ImagenID] = f
	}
	if len(req.FotosIDs) != len(b.Photos) {
		return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "fotos_ids", Message: "deben venir todas las fotos de la galería"})
	}
	fotos := make([]negocios.Foto, 0, len(req.FotosIDs))
	for i, fid := range req.FotosIDs {
		f, ok := porID[fid]
		if !ok || fid == "" {
			return NegocioResponse{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "fotos_ids", Message: "deben ser las fotos de la galería, sin repetir"})
		}
		delete(porID, fid)
		f.Orden = i
		fotos = append(fotos, f)
	}
	return s.guardarFotos(ctx, b, fotos)
}

func (s *Service) guardarFotos(ctx context.Context, b negocios.Business, fotos []negocios.Foto) (NegocioResponse, error) {
	nb, err := s.repo.ApplyIf(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "version", Value: b.Version}}, bson.D{{Key: "fotos", Value: fotos}})
	if errors.Is(err, negocios.ErrNoAplica) {
		return NegocioResponse{}, errVersion()
	}
	if err != nil {
		return NegocioResponse{}, errStore()
	}
	if nb.State == negocios.StatusApproved && s.fichas != nil {
		s.fichas.InvalidarFicha(ctx, nb.ID.Hex())
	}
	return negocioResponse(nb), nil
}

// SubirDocumento cifra y guarda un documento del expediente (INE o comprobante).
func (s *Service) SubirDocumento(ctx context.Context, claims *middleware.Claims, id, tipo string, datos []byte) (expediente.Metadato, error) {
	owner, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return expediente.Metadato{}, err
	}
	if err := editable(b); err != nil {
		return expediente.Metadato{}, err
	}
	if err := s.consentimiento(ctx, owner); err != nil {
		return expediente.Metadato{}, err
	}
	meta, err := s.exp.SubirArchivo(ctx, b.ID, owner, tipo, datos)
	if err != nil {
		return expediente.Metadato{}, err
	}
	vigentes, err := s.exp.Vigentes(ctx, b.ID)
	if err != nil {
		return expediente.Metadato{}, err
	}
	flags := negocios.Documentos{
		INE:         vigentes[expediente.TipoINEFrente] && vigentes[expediente.TipoINEReverso],
		Comprobante: vigentes[expediente.TipoComprobante],
	}
	if err := s.repo.SetDocumentos(ctx, b.ID, flags); err != nil {
		return expediente.Metadato{}, errStore()
	}
	return meta, nil
}

// Documentos lista los metadatos del expediente (acceso de ARCO del dueño).
func (s *Service) Documentos(ctx context.Context, claims *middleware.Claims, id string) (DocumentosResponse, error) {
	_, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return DocumentosResponse{}, err
	}
	items, err := s.exp.Listar(ctx, b.ID)
	if err != nil {
		return DocumentosResponse{}, err
	}
	return DocumentosResponse{Items: items}, nil
}

// LeerDocumento descifra un documento para su dueño (sin bitácora).
func (s *Service) LeerDocumento(ctx context.Context, claims *middleware.Claims, id, tipo string) (expediente.Contenido, error) {
	_, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return expediente.Contenido{}, err
	}
	return s.exp.LeerDueno(ctx, b.ID, tipo)
}

// GuardarFiscales guarda los datos fiscales cifrados. Se aceptan en cualquier estado: nv-59
// los pide al activar anticipos.
func (s *Service) GuardarFiscales(ctx context.Context, claims *middleware.Claims, id string, req FiscalesRequest) (expediente.Metadato, error) {
	owner, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return expediente.Metadato{}, err
	}
	if err := s.consentimiento(ctx, owner); err != nil {
		return expediente.Metadato{}, err
	}
	return s.exp.GuardarFiscales(ctx, b.ID, owner, expediente.DatosFiscales{
		RFC: req.RFC, RazonSocial: req.RazonSocial, RegimenFiscal: req.RegimenFiscal, UsoCFDI: req.UsoCFDI, CPFiscal: req.CPFiscal,
	})
}

// Estados de la cuenta de depósito: `sin_verificar` hasta que nv-60 la registra en Stripe.
const (
	EstadoSinVerificar = "sin_verificar"
	EstadoRegistrada   = "registrada"
)

// GuardarCuenta guarda la CLABE cifrada y deja en `negocios` solo la versión enmascarada.
func (s *Service) GuardarCuenta(ctx context.Context, claims *middleware.Claims, id string, req CuentaRequest) (negocios.CuentaMascara, error) {
	owner, b, err := s.propio(ctx, claims, id)
	if err != nil {
		return negocios.CuentaMascara{}, err
	}
	if err := s.consentimiento(ctx, owner); err != nil {
		return negocios.CuentaMascara{}, err
	}
	c, err := s.exp.GuardarCuenta(ctx, b.ID, owner, req.CLABE, req.Titular)
	if err != nil {
		return negocios.CuentaMascara{}, err
	}
	m := negocios.CuentaMascara{Ultimos4: expediente.Enmascarar(c.CLABE), Banco: c.Banco, Titular: c.Titular, Estado: EstadoSinVerificar}
	if s.deposito != nil {
		// Si Stripe falla, la cuenta queda sin verificar y se registra al conectar los cobros.
		externa, err := s.deposito.RegistrarCuenta(ctx, b, c.CLABE, c.Titular)
		if err != nil {
			s.log.Warn("alta: la cuenta de depósito no se registró en Stripe", "evento", "cuenta_stripe_fallo", "error", err.Error())
		} else if externa != "" {
			m.Estado, m.StripeExternalAccountID = EstadoRegistrada, externa
		}
	}
	if err := s.repo.SetCuentaDeposito(ctx, b.ID, m); err != nil {
		return negocios.CuentaMascara{}, errStore()
	}
	return m, nil
}

// LeerAdmin entrega un documento al administrador después de registrar la lectura.
func (s *Service) LeerAdmin(ctx context.Context, claims *middleware.Claims, id, tipo, motivo, ip, requestID string) (expediente.Contenido, error) {
	admin, err := sujeto(claims)
	if err != nil {
		return expediente.Contenido{}, err
	}
	nid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return expediente.Contenido{}, errNoEncontrado()
	}
	// Un negocio dado de baja se sigue pudiendo leer: los fiscales bloqueados viven 5 años.
	if _, err := s.repo.FindByID(ctx, nid); err != nil {
		if errors.Is(err, negocios.ErrNotFound) {
			return expediente.Contenido{}, errNoEncontrado()
		}
		return expediente.Contenido{}, errStore()
	}
	return s.exp.LeerAdmin(ctx, expediente.Acceso{AdminID: admin, NegocioID: nid, Tipo: tipo, Motivo: motivo, IP: ip, RequestID: requestID})
}
