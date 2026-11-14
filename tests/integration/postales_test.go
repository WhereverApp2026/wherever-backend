package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/rekognitiontest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

const postalesPath = "/api/v1/postales"

// postalesEnv deja el entorno con un negocio aprobado de un comerciante.
type postalesEnv struct {
	env     *authtest.Env
	app     *fiber.App
	negocio negocios.Business
	dueno   string        // token del comerciante
	lugar   bson.ObjectID // abierto toda la semana, para las paradas de los planes
}

func nuevoPostales(t *testing.T) postalesEnv {
	t.Helper()
	env, app := amistadesEnv(t)
	b, tok := dueno(t, env, "duena-postales@wherever.test", reservastest.Config{})
	lugar := placeDoc(placeSpec{slug: "postales-plan"})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), lugar)
	require.NoError(t, err)
	return postalesEnv{env: env, app: app, negocio: b, dueno: tok, lugar: lugar.ID}
}

// pcall manda la petición, revisa el estado y devuelve el cuerpo (snake_case medido).
func pcall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int, headers ...string) map[string]any {
	t.Helper()
	h := map[string]string{}
	for i := 0; i+1 < len(headers); i += 2 {
		h[headers[i]] = headers[i+1]
	}
	var resp *http.Response
	if len(h) == 0 {
		resp = sendJSON(t, app, method, path, body, token)
	} else {
		resp = sendWithHeaders(t, app, method, path, body, token, h)
	}
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	if raw == "" || want == http.StatusNoContent || want == http.StatusAccepted {
		return nil
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assertSnakeCase(t, out)
	return out
}

// fotoPublicada sube y confirma una foto aprobada por el Rekognition local.
func fotoPublicada(t *testing.T, s postalesEnv, who amigo) string {
	t.Helper()
	s.env.Imagenes.Rekognition.Responder()
	id := subirFoto(t, s.env, s.app, who.token, "image/jpeg", fotoJPEG(t, 64, 64))
	out := imgCall(t, s.app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, who.token, http.StatusOK)
	require.Equal(t, imagenes.EstadoPublicada, out["estado"])
	return id
}

// fotoDudosa queda en revisión (cuarentena de nv-45).
func fotoDudosa(t *testing.T, s postalesEnv, who amigo) string {
	t.Helper()
	s.env.Imagenes.Rekognition.Responder(rekognitiontest.Etiqueta{Name: "Violence", Confidence: 55, TaxonomyLevel: 1})
	id := subirFoto(t, s.env, s.app, who.token, "image/jpeg", fotoJPEG(t, 64, 64))
	out := imgCall(t, s.app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, who.token, http.StatusOK)
	require.Equal(t, imagenes.EstadoEnRevision, out["estado"])
	s.env.Imagenes.Rekognition.Responder()
	return id
}

// crearPostal crea la postal y devuelve su id (sale de `Location`).
func crearPostal(t *testing.T, s postalesEnv, who amigo, body map[string]any) string {
	t.Helper()
	resp := sendJSON(t, s.app, http.MethodPost, postalesPath, body, who.token)
	requireStatus(t, resp, http.StatusAccepted)
	assert.Empty(t, readAll(t, resp), "202 sin cuerpo")
	loc := resp.Header.Get(fiber.HeaderLocation)
	require.Contains(t, loc, postalesPath+"/")
	return loc[len(postalesPath)+1:]
}

func cuerpoPostal(img, vis string) map[string]any {
	return map[string]any{"imagen_id": img, "estilo": postales.EstiloPolaroid, "texto": "Gran tarde", "visibilidad": vis}
}

func ifm(v int) []string {
	return []string{fiber.HeaderIfMatch, `"` + strconv.Itoa(v) + `"`}
}

func bloquear(t *testing.T, s postalesEnv, a, b amigo) {
	t.Helper()
	_, err := s.env.Mongo.DB().Collection(amistades.Collection).DeleteMany(context.Background(), bson.D{{Key: "par", Value: amistades.PairKey(a.user.ID, b.user.ID)}})
	require.NoError(t, err)
	befriend(t, s.env, a, b)
	_, err = s.env.Mongo.DB().Collection(amistades.Collection).UpdateOne(context.Background(), bson.D{{Key: "par", Value: amistades.PairKey(a.user.ID, b.user.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: amistades.StatusBlocked}}}})
	require.NoError(t, err)
}

func avisos(t *testing.T, s postalesEnv, to amigo, tipo string) int64 {
	t.Helper()
	n, err := s.env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: to.user.ID}, {Key: "tipo", Value: tipo}})
	require.NoError(t, err)
	return n
}

func TestPostales_CrearConImagenPropiaAjenaOUsada(t *testing.T) {
	s := nuevoPostales(t)
	yo, otra := seedAmigo(t, s.env, "postalera"), seedAmigo(t, s.env, "ajena")
	img := fotoPublicada(t, s, yo)
	body := cuerpoPostal(img, postales.VisPrivada)
	body["negocio_id"] = s.negocio.ID.Hex()
	id := crearPostal(t, s, yo, body)

	got := pcall(t, s.app, http.MethodGet, postalesPath+"/"+id, nil, yo.token, http.StatusOK)
	assert.Equal(t, postales.EstadoLista, got["estado"])
	assert.Equal(t, img, got["imagen_id"])
	assert.Contains(t, got["imagen_url"], "/i/"+img+"/1080.jpg")
	assert.NotContains(t, got["imagen_url"], "cuarentena")
	assert.Equal(t, true, got["propia"])

	pcall(t, s.app, http.MethodPost, postalesPath, body, otra.token, http.StatusNotFound) // imagen ajena
	pcall(t, s.app, http.MethodPost, postalesPath, body, yo.token, http.StatusConflict)   // ya usada
	pendiente := str(t, imgCall(t, s.app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, yo.token, http.StatusCreated)["id"])
	pcall(t, s.app, http.MethodPost, postalesPath, cuerpoPostal(pendiente, postales.VisPrivada), yo.token, http.StatusUnprocessableEntity)
	otraFoto := fotoPublicada(t, s, yo)
	malo := cuerpoPostal(otraFoto, postales.VisPrivada)
	malo["negocio_id"] = bson.NewObjectID().Hex()
	pcall(t, s.app, http.MethodPost, postalesPath, malo, yo.token, http.StatusNotFound)

	s.env.ModeradorPostales.Fijar(moderacion.VeredictoRechazado)
	pcall(t, s.app, http.MethodPost, postalesPath, cuerpoPostal(otraFoto, postales.VisPrivada), yo.token, http.StatusUnprocessableEntity)
	assert.Contains(t, s.env.Logs.String(), "postal_creada")
	assert.NotContains(t, s.env.Logs.String(), "Gran tarde", "nunca el texto en los logs")
	assert.NotContains(t, s.env.Logs.String(), "1080.jpg", "ni URLs")
}

func TestPostales_VisibilidadYBloqueos(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga, extrano := seedAmigo(t, s.env, "duena-postal"), seedAmigo(t, s.env, "amiga-postal"), seedAmigo(t, s.env, "extrano-postal")
	befriend(t, s.env, yo, amiga)
	id := crearPostal(t, s, yo, cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPrivada))
	ruta := postalesPath + "/" + id

	pcall(t, s.app, http.MethodGet, ruta, nil, amiga.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodGet, ruta, nil, "", http.StatusUnauthorized)
	pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"visibilidad": "amigos"}, yo.token, http.StatusOK, ifm(1)...)
	vista := pcall(t, s.app, http.MethodGet, ruta, nil, amiga.token, http.StatusOK)
	assert.Nil(t, vista["imagen_id"], "solo el dueño ve el id de la imagen")
	assert.Equal(t, "duena-postal", obj(t, vista["autor"])["nombre_usuario"])
	assert.Nil(t, obj(t, vista["autor"])["correo"])
	pcall(t, s.app, http.MethodGet, ruta, nil, extrano.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"visibilidad": "publica"}, yo.token, http.StatusOK, ifm(2)...)
	pcall(t, s.app, http.MethodGet, ruta, nil, extrano.token, http.StatusOK)
	bloquear(t, s, yo, extrano)
	pcall(t, s.app, http.MethodGet, ruta, nil, extrano.token, http.StatusNotFound)

	// Una foto dudosa deja la postal en moderación: nadie más la ve hasta que se apruebe.
	dudosa := fotoDudosa(t, s, yo)
	pid := crearPostal(t, s, yo, cuerpoPostal(dudosa, postales.VisPublica))
	propia := pcall(t, s.app, http.MethodGet, postalesPath+"/"+pid, nil, yo.token, http.StatusOK)
	assert.Equal(t, postales.EstadoEnModeracion, propia["estado"])
	assert.Nil(t, propia["imagen_url"], "nunca la ruta de cuarentena")
	pcall(t, s.app, http.MethodGet, postalesPath+"/"+pid, nil, amiga.token, http.StatusNotFound)
	decidirCola(t, s.env, dudosa, "aprobado")
	lista := pcall(t, s.app, http.MethodGet, postalesPath+"/"+pid, nil, amiga.token, http.StatusOK)
	assert.Equal(t, postales.EstadoLista, lista["estado"])
	assert.Contains(t, lista["imagen_url"], "/i/"+dudosa+"/1080.jpg")
}

func TestPostales_AlbumFiltrosEIfMatch(t *testing.T) {
	s := nuevoPostales(t)
	yo := seedAmigo(t, s.env, "album")
	a := crearPostal(t, s, yo, cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPrivada))
	b := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisAmigos)
	b["estilo"] = postales.EstiloPasaporte
	b["negocio_id"] = s.negocio.ID.Hex()
	crearPostal(t, s, yo, b)

	todas := list(t, pcall(t, s.app, http.MethodGet, postalesPath+"?limite=1", nil, yo.token, http.StatusOK)["items"])
	require.Len(t, todas, 1)
	pas := list(t, pcall(t, s.app, http.MethodGet, postalesPath+"?estilo=pasaporte", nil, yo.token, http.StatusOK)["items"])
	require.Len(t, pas, 1)
	neg := list(t, pcall(t, s.app, http.MethodGet, postalesPath+"?negocio_id="+s.negocio.ID.Hex(), nil, yo.token, http.StatusOK)["items"])
	require.Len(t, neg, 1)
	assert.Empty(t, list(t, pcall(t, s.app, http.MethodGet, postalesPath+"?mes=2001-01", nil, yo.token, http.StatusOK)["items"]))
	pcall(t, s.app, http.MethodGet, postalesPath+"?mes=enero", nil, yo.token, http.StatusUnprocessableEntity)

	ruta := postalesPath + "/" + a
	pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"estilo": "clasica"}, yo.token, http.StatusBadRequest)
	out := pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"estilo": "clasica", "texto": "Otra"}, yo.token, http.StatusOK, ifm(1)...)
	assert.Equal(t, float64(2), out["version"])
	assert.Equal(t, "Otra", out["texto"])
	pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"estilo": "polaroid"}, yo.token, http.StatusConflict, ifm(1)...)
	pcall(t, s.app, http.MethodPatch, ruta, map[string]any{"estilo": "polaroid"}, seedAmigo(t, s.env, "intrusa").token, http.StatusNotFound, ifm(2)...)
}

func TestPostales_BorrarConMomentoYGaleria(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga := seedAmigo(t, s.env, "galeria"), seedAmigo(t, s.env, "galeria-amiga")
	befriend(t, s.env, yo, amiga)
	body := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPublica)
	body["negocio_id"] = s.negocio.ID.Hex()
	id := crearPostal(t, s, yo, body)
	dudosa := cuerpoPostal(fotoDudosa(t, s, yo), postales.VisPublica)
	dudosa["negocio_id"] = s.negocio.ID.Hex()
	crearPostal(t, s, yo, dudosa)
	privada := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisPrivada)
	privada["negocio_id"] = s.negocio.ID.Hex()
	crearPostal(t, s, yo, privada)

	galeria := "/api/v1/lugares/" + s.negocio.ID.Hex() + "/postales"
	items := list(t, pcall(t, s.app, http.MethodGet, galeria, nil, "", http.StatusOK)["items"])
	require.Len(t, items, 1, "solo publica y lista")
	item := obj(t, items[0])
	assert.Equal(t, id, item["id"])
	assert.Equal(t, "galeria", obj(t, item["autor"])["nombre_usuario"])
	assert.Nil(t, item["plan_id"])
	pcall(t, s.app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/postales", nil, "", http.StatusNotFound)

	m := pcall(t, s.app, http.MethodPost, "/api/v1/momentos", map[string]any{"postal_id": id, "visibilidad": "publica"}, yo.token, http.StatusCreated)
	mid := str(t, m["id"])
	pcall(t, s.app, http.MethodDelete, postalesPath+"/"+id, nil, yo.token, http.StatusBadRequest)
	pcall(t, s.app, http.MethodDelete, postalesPath+"/"+id, nil, yo.token, http.StatusNoContent, ifm(1)...)
	pcall(t, s.app, http.MethodGet, "/api/v1/momentos/"+mid, nil, yo.token, http.StatusNotFound)
	pcall(t, s.app, http.MethodDelete, postalesPath+"/"+id, nil, yo.token, http.StatusNotFound, ifm(1)...)
	assert.Empty(t, list(t, pcall(t, s.app, http.MethodGet, galeria, nil, "", http.StatusOK)["items"]))
}

func TestPostales_ARCO(t *testing.T) {
	s := nuevoPostales(t)
	yo, amiga := seedAmigo(t, s.env, "arco-postal"), seedAmigo(t, s.env, "arco-amiga")
	befriend(t, s.env, yo, amiga)
	plan := planPasado(t, s, yo)
	body := cuerpoPostal(fotoPublicada(t, s, yo), postales.VisAmigos)
	body["plan_id"] = plan
	id := crearPostal(t, s, yo, body)
	pcall(t, s.app, http.MethodPost, "/api/v1/momentos", map[string]any{"postal_id": id, "visibilidad": "amigos", "texto": "Mi texto"}, yo.token, http.StatusCreated)
	pcall(t, s.app, http.MethodPost, "/api/v1/planes/"+plan+"/recuerdo", nil, yo.token, http.StatusCreated)
	ajena := crearPostal(t, s, amiga, cuerpoPostal(fotoPublicada(t, s, amiga), postales.VisAmigos))
	ajeno := pcall(t, s.app, http.MethodPost, "/api/v1/momentos", map[string]any{"postal_id": ajena, "visibilidad": "amigos", "texto": "Texto de ella", "etiquetados": []string{yo.id()}}, amiga.token, http.StatusCreated)

	repo := postales.NewRepository(s.env.Mongo.DB())
	ctx := context.Background()
	exp, err := postales.NewPrivacy(repo).Export(ctx, yo.user.ID)
	require.NoError(t, err)
	raw, _ := json.Marshal(exp)
	assert.Contains(t, string(raw), "/1080.jpg")
	exp, err = postales.NewRecuerdosPrivacy(repo).Export(ctx, yo.user.ID)
	require.NoError(t, err)
	raw, _ = json.Marshal(exp)
	assert.Contains(t, string(raw), plan)
	exp, err = postales.NewMomentosPrivacy(repo).Export(ctx, yo.user.ID)
	require.NoError(t, err)
	raw, _ = json.Marshal(exp)
	assert.Contains(t, string(raw), "Mi texto")
	assert.Contains(t, string(raw), str(t, ajeno["id"]), "el id del momento donde la etiquetaron")
	assert.NotContains(t, string(raw), "Texto de ella", "nunca contenido ajeno")

	p := postales.NewPrivacy(repo)
	require.NoError(t, p.PurgeHistory(ctx, yo.user.ID))
	ps, err := repo.PostalesDe(ctx, yo.user.ID)
	require.NoError(t, err)
	require.Len(t, ps, 1, "borrar historial no toca el contenido")
	require.NoError(t, p.PurgeAccount(ctx, yo.user.ID))
	for _, n := range []func() (int, error){
		func() (int, error) { x, err := repo.PostalesDe(ctx, yo.user.ID); return len(x), err },
		func() (int, error) { x, err := repo.RecuerdosDe(ctx, yo.user.ID); return len(x), err },
		func() (int, error) { x, err := repo.MomentosDe(ctx, yo.user.ID); return len(x), err },
		func() (int, error) { x, err := repo.EtiquetadoEn(ctx, yo.user.ID); return len(x), err },
	} {
		got, err := n()
		require.NoError(t, err)
		assert.Zero(t, got)
	}
	pcall(t, s.app, http.MethodGet, "/api/v1/momentos/"+str(t, ajeno["id"]), nil, amiga.token, http.StatusOK)
}
