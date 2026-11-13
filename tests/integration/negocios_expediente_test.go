package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const clabeValida = "012180000118359713"

func subirMultipart(t *testing.T, app *fiber.App, path, tipo string, datos []byte, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("tipo", tipo))
	part, err := w.CreateFormFile("archivo", "foto.jpg")
	require.NoError(t, err)
	_, err = part.Write(datos)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set(fiber.HeaderContentType, w.FormDataContentType())
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func admin(u bson.ObjectID) *middleware.Claims {
	return authtest.ClaimsDe(u.Hex(), middleware.RoleAdmin)
}

func TestNegociosExpediente_CifradoReemplazoYConsentimiento(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	u := env.CreateUser(t, "duena-exp@wherever.test")
	b, err := env.Alta.Crear(ctx, authtest.Comerciante(u), alta.CrearRequest{Nombre: "Bar Nube", Categoria: "bar"})
	require.NoError(t, err)
	access, _ := loginHTTP(t, app, u.Email) // ya con el rol comerciante
	path := altaPath + "/" + b.ID + "/documentos"
	img := authtest.PNG(t)

	resp := subirMultipart(t, app, path, expediente.TipoINEFrente, img, access)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "consentimiento_requerido")

	env.ConsentirDatosNegocio(t, u)
	resp = subirMultipart(t, app, path, expediente.TipoINEFrente, img, access)
	requireStatus(t, resp, http.StatusCreated)
	claves := env.Almacen.Claves()
	require.Len(t, claves, 1)
	assert.True(t, strings.HasPrefix(claves[0], "privado/negocios/"+b.ID+"/ine_frente/"))
	guardado, _ := env.Almacen.Objeto(claves[0])
	assert.False(t, bytes.Contains(guardado, img[8:]), "en S3 solo hay bytes cifrados")

	// Reemplazar borra el objeto anterior.
	resp = subirMultipart(t, app, path, expediente.TipoINEFrente, img, access)
	requireStatus(t, resp, http.StatusCreated)
	require.Len(t, env.Almacen.Claves(), 1)
	assert.NotEqual(t, claves[0], env.Almacen.Claves()[0])

	// Otro tipo de archivo con extensión .jpg = 422.
	resp = subirMultipart(t, app, path, expediente.TipoINEReverso, []byte("<html><script>x</script></html>"), access)
	requireStatus(t, resp, http.StatusUnprocessableEntity)

	// El dueño lo lee descifrado, sin caché ni compresión.
	req := httptest.NewRequest(http.MethodGet, path+"/ine_frente", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+access)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
	got, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = got.Body.Close() }()
	require.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, "no-store", got.Header.Get(fiber.HeaderCacheControl))
	assert.Empty(t, got.Header.Get(fiber.HeaderContentEncoding))
	assert.Contains(t, got.Header.Get(fiber.HeaderContentDisposition), "attachment")
	raw, err := io.ReadAll(got.Body)
	require.NoError(t, err)
	assert.Equal(t, "image/png", got.Header.Get(fiber.HeaderContentType))
	assert.NotEmpty(t, raw)

	// Fiscales y CLABE: validados y cifrados; nada en claro en MongoDB.
	resp = sendJSON(t, app, http.MethodPut, altaPath+"/"+b.ID+"/datos-fiscales", map[string]any{"rfc": "XXXX", "razon_social": "Nube SA", "regimen_fiscal": "601", "uso_cfdi": "G03", "cp_fiscal": "06600"}, access)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "rfc_invalido")
	resp = sendJSON(t, app, http.MethodPut, altaPath+"/"+b.ID+"/datos-fiscales", map[string]any{"rfc": "ABC680524P76", "razon_social": "Nube SA de CV", "regimen_fiscal": "601", "uso_cfdi": "G03", "cp_fiscal": "06600"}, access)
	requireStatus(t, resp, http.StatusOK)
	resp = sendJSON(t, app, http.MethodPut, altaPath+"/"+b.ID+"/cuenta-deposito", map[string]any{"clabe": "012180000118359714", "titular": "Ana Pérez"}, access)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "clabe_invalida")
	resp = sendJSON(t, app, http.MethodPut, altaPath+"/"+b.ID+"/cuenta-deposito", map[string]any{"clabe": clabeValida, "titular": "Ana Pérez"}, access)
	requireStatus(t, resp, http.StatusOK)
	cuenta := decode(t, resp)
	assert.Equal(t, "9713", cuenta["clabe_ultimos4"])
	assert.Equal(t, "BBVA MEXICO", cuenta["banco"])

	var docs []bson.M
	cur, err := env.Mongo.DB().Collection(expediente.Collection).Find(ctx, bson.D{})
	require.NoError(t, err)
	require.NoError(t, cur.All(ctx, &docs))
	var negocio bson.M
	require.NoError(t, env.Mongo.DB().Collection("negocios").FindOne(ctx, bson.D{{Key: "_id", Value: mustOID(t, b.ID)}}).Decode(&negocio))
	for _, d := range append(docs, negocio) {
		dump := fmt.Sprint(d)
		assert.NotContains(t, dump, clabeValida)
		assert.NotContains(t, dump, "ABC680524P76")
		assert.NotContains(t, dump, "Nube SA de CV")
	}
	assert.NotContains(t, env.Logs.String(), clabeValida, "ni en los logs")
	assert.NotContains(t, env.Logs.String(), "ABC680524P76")
}

func TestNegociosExpediente_SinDataKeys503(t *testing.T) {
	cfg := testutil.Config("dev")
	cfg.DataKeys = ""
	env := authEnvWith(t, cfg)
	ctx := context.Background()
	u := env.CreateUser(t, "duena-sin-llave@wherever.test")
	cl := authtest.Comerciante(u)
	b, err := env.Alta.Crear(ctx, cl, alta.CrearRequest{Nombre: "Sin llave", Categoria: "bar"})
	require.NoError(t, err)
	env.ConsentirDatosNegocio(t, u)
	_, err = env.Alta.SubirDocumento(ctx, cl, b.ID, expediente.TipoINEFrente, authtest.PNG(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_disponible")
	_, err = env.Alta.GuardarCuenta(ctx, cl, b.ID, alta.CuentaRequest{CLABE: clabeValida, Titular: "Ana"})
	assert.Contains(t, err.Error(), "no_disponible")
	assert.Empty(t, env.Almacen.Claves(), "nunca se guarda en claro")
}

func TestNegociosExpediente_LecturaAdminConBitacora(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	a := env.CreateAdmin(t, "admin-exp@wherever.test")
	adminToken, _ := loginHTTP(t, app, a.Email)
	u := env.CreateUser(t, "duena-bitacora@wherever.test")
	b := env.AltaEnRevision(t, u, "Local auditado")
	path := "/api/v1/admin/negocios/" + b.ID + "/documentos/ine_frente"

	resp := sendJSON(t, app, http.MethodGet, path, nil, adminToken)
	requireStatus(t, resp, http.StatusUnprocessableEntity) // sin motivo
	resp = sendJSON(t, app, http.MethodGet, path+"?motivo=Revision+del+alta+del+negocio", nil, adminToken)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))

	accesos, err := expediente.NewRepository(env.Mongo.DB()).Accesos(ctx, mustOID(t, b.ID))
	require.NoError(t, err)
	require.Len(t, accesos, 1)
	assert.Equal(t, a.ID, accesos[0].AdminID)
	assert.Equal(t, "Revision del alta del negocio", accesos[0].Motivo)
	assert.NotEmpty(t, accesos[0].RequestID)

	// Bitácora caída: no se entrega el archivo (fail-closed).
	require.NoError(t, env.Mongo.DB().Collection(expediente.AccesosCollection).Drop(ctx))
	_, err = env.Mongo.DB().RunCommand(ctx, bson.D{
		{Key: "create", Value: expediente.AccesosCollection},
		{Key: "validator", Value: bson.D{{Key: "nunca", Value: bson.D{{Key: "$exists", Value: true}}}}},
		{Key: "validationAction", Value: "error"},
	}).Raw()
	require.NoError(t, err)
	_, err = env.Alta.LeerAdmin(ctx, admin(a.ID), b.ID, expediente.TipoINEFrente, "Revision del alta del negocio", "203.0.113.9", "req-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_disponible")
}

func TestNegociosExpediente_RetencionYArco(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	a := env.CreateAdmin(t, "admin-retencion@wherever.test")
	u := env.CreateUser(t, "duena-retencion@wherever.test")
	cl := authtest.Comerciante(u)
	b := env.AltaEnRevision(t, u, "Local retenido")
	_, err := env.Alta.GuardarFiscales(ctx, cl, b.ID, alta.FiscalesRequest{RFC: "GODE561231GR8", RazonSocial: "Elena Gómez", RegimenFiscal: "612", UsoCFDI: "G03", CPFiscal: "06600"})
	require.NoError(t, err)
	_, err = env.Alta.GuardarCuenta(ctx, cl, b.ID, alta.CuentaRequest{CLABE: clabeValida, Titular: "Elena Gómez"})
	require.NoError(t, err)
	_, err = env.Alta.Decidir(ctx, admin(a.ID), b.ID, b.Version, alta.DecisionRequest{Decision: alta.DecisionAprobar})
	require.NoError(t, err)
	require.Len(t, env.Almacen.Claves(), 3)

	// Antes de 90 días no se borra nada; después, la INE y el comprobante sí.
	env.Expediente.WithClock(func() time.Time { return time.Now().Add(89 * 24 * time.Hour) })
	n, err := env.Expediente.Retencion(ctx, 100)
	require.NoError(t, err)
	assert.Zero(t, n)
	env.Expediente.WithClock(func() time.Time { return time.Now().Add(91 * 24 * time.Hour) })
	n, err = env.Expediente.Retencion(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Empty(t, env.Almacen.Claves(), "borrado real en S3")
	env.Expediente.WithClock(time.Now)

	// Exportación: fiscales y CLABE enmascarada, nunca la CLABE completa.
	export, err := expediente.NewPrivacy(env.Expediente).Export(ctx, u.ID)
	require.NoError(t, err)
	dump := fmt.Sprint(export)
	assert.Contains(t, dump, "9713")
	assert.NotContains(t, dump, clabeValida)
	assert.Contains(t, dump, "GODE561231GR8")

	// Cancelación: borra la CLABE y bloquea los fiscales 5 años (solo el admin los lee).
	require.NoError(t, expediente.NewPrivacy(env.Expediente).PurgeAccount(ctx, u.ID))
	_, err = env.Alta.LeerDocumento(ctx, cl, b.ID, expediente.TipoCuenta)
	require.Error(t, err)
	_, err = env.Alta.LeerDocumento(ctx, cl, b.ID, expediente.TipoFiscales)
	require.Error(t, err, "bloqueados para el dueño")
	got, err := env.Alta.LeerAdmin(ctx, admin(a.ID), b.ID, expediente.TipoFiscales, "Requerimiento del SAT por factura", "203.0.113.9", "req-2")
	require.NoError(t, err)
	assert.Contains(t, string(got.Datos), "GODE561231GR8")

	env.Expediente.WithClock(func() time.Time { return time.Now().Add(5*365*24*time.Hour + 24*time.Hour) })
	n, err = env.Expediente.Retencion(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "los fiscales se borran a los 5 años")
	env.Expediente.WithClock(time.Now)

	// Retirar el consentimiento es la oposición: borra el expediente del negocio.
	u2 := env.CreateUser(t, "duena-oposicion@wherever.test")
	b2 := env.AltaCompleta(t, u2, "Local opuesto")
	require.NotEmpty(t, env.Almacen.Claves())
	retirarDatosNegocio(t, env, u2.ID)
	meta, err := env.Expediente.Listar(ctx, mustOID(t, b2.ID))
	require.NoError(t, err)
	assert.Empty(t, meta)
}

func retirarDatosNegocio(t *testing.T, env *authtest.Env, id bson.ObjectID) {
	t.Helper()
	u, err := env.Users.FindByID(context.Background(), id)
	require.NoError(t, err)
	app := env.App()
	access, _ := loginHTTP(t, app, u.Email)
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/consentimientos", map[string]any{
		"version_terminos": env.Cfg.VersionApp.Terminos, "version_aviso_privacidad": env.Cfg.VersionApp.Aviso, "datos_negocio": false,
	}, access)
	requireStatus(t, resp, http.StatusCreated)
}
