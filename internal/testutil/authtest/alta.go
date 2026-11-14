package authtest

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Comerciante son los claims de la persona con rol comerciante (como tras el refresh).
func Comerciante(u usuarios.User) *middleware.Claims {
	return ClaimsDe(u.ID.Hex(), middleware.RoleConsumer, middleware.RoleMerchant)
}

// ClaimsDe arma los claims de un sujeto con esos roles, para llamar al service directo.
func ClaimsDe(subject string, roles ...string) *middleware.Claims {
	c := &middleware.Claims{Roles: roles}
	c.Subject = subject
	return c
}

// ConsentirDatosNegocio registra el consentimiento `datos_negocio` de la versión vigente.
func (e *Env) ConsentirDatosNegocio(t testing.TB, u usuarios.User) {
	t.Helper()
	_, err := e.Users.Update(context.Background(), u.ID, bson.D{
		{Key: "privacidad.datos_negocio", Value: true},
		{Key: "privacidad.version_aviso", Value: e.Cfg.VersionApp.Aviso},
	})
	require.NoError(t, err)
}

// PNG es una imagen válida pequeña para los documentos.
func PNG(t testing.TB) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{G: 200, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// AltaCompleta lleva un negocio de la persona por los pasos 1 a 6, con 3 fotos, INE,
// comprobante y consentimiento, sin enviarlo a revisión.
func (e *Env) AltaCompleta(t testing.TB, u usuarios.User, nombre string) alta.NegocioResponse {
	t.Helper()
	ctx := context.Background()
	cl := Comerciante(u)
	b, err := e.Alta.Crear(ctx, cl, alta.CrearRequest{Nombre: nombre, Categoria: "restaurante", Descripcion: "Comida casera"})
	require.NoError(t, err)
	b, err = e.Alta.GuardarPaso(ctx, cl, b.ID, b.Version, alta.Paso{Numero: 2, Dos: &alta.Paso2Request{
		Direccion: negocios.Direccion{Calle: "Juárez", Numero: "10", Colonia: "Centro", Municipio: "Cuauhtémoc", Estado: "CDMX", CP: "06000"},
		Ubicacion: alta.Ubicacion{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}},
	}})
	require.NoError(t, err)
	b, err = e.Alta.GuardarPaso(ctx, cl, b.ID, b.Version, alta.Paso{Numero: 3, Tres: &alta.Paso3Request{
		Horarios: []negocios.Horario{{Dia: 1, Abre: "09:00", Cierra: "18:00"}},
	}})
	require.NoError(t, err)
	for i, tipo := range []string{"fachada", "interior", "producto"} {
		id := b.ID + "-foto-" + string(rune('a'+i))
		e.ImagenesNegocio.Publicar(u.ID, id)
		b, err = e.Alta.AgregarFoto(ctx, cl, b.ID, alta.FotoRequest{ImagenID: id, Tipo: tipo})
		require.NoError(t, err)
	}
	b, err = e.Alta.GuardarPaso(ctx, cl, b.ID, b.Version, alta.Paso{Numero: 4})
	require.NoError(t, err)
	b, err = e.Alta.GuardarPaso(ctx, cl, b.ID, b.Version, alta.Paso{Numero: 5, Cinco: &alta.Paso5Request{
		NivelPrecio: 2, PrecioPromedio: 150, Servicios: []string{"terraza"}, Alergenos: []string{negocios.AllergenNone},
	}})
	require.NoError(t, err)
	e.ConsentirDatosNegocio(t, u)
	for _, tipo := range expediente.TiposArchivo {
		_, err = e.Alta.SubirDocumento(ctx, cl, b.ID, tipo, PNG(t))
		require.NoError(t, err)
	}
	b, err = e.Alta.GuardarPaso(ctx, cl, b.ID, b.Version, alta.Paso{Numero: 6})
	require.NoError(t, err)
	return b
}

// AltaEnRevision es AltaCompleta más el envío a revisión.
func (e *Env) AltaEnRevision(t testing.TB, u usuarios.User, nombre string) alta.NegocioResponse {
	t.Helper()
	b := e.AltaCompleta(t, u, nombre)
	out, err := e.Alta.EnviarRevision(context.Background(), Comerciante(u), b.ID)
	require.NoError(t, err)
	return out
}

// AltaAprobada es AltaEnRevision más la aprobación de un administrador (nv-56: el panel parte
// de un negocio aprobado).
func (e *Env) AltaAprobada(t testing.TB, u usuarios.User, nombre string) alta.NegocioResponse {
	t.Helper()
	b := e.AltaEnRevision(t, u, nombre)
	out, err := e.Alta.Decidir(context.Background(), ClaimsDe(bson.NewObjectID().Hex(), middleware.RoleAdmin), b.ID, b.Version, alta.DecisionRequest{Decision: alta.DecisionAprobar})
	require.NoError(t, err)
	return out
}
