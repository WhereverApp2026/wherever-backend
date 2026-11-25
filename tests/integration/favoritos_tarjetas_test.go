package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

const lugaresCercaPath = "/api/v1/lugares?lat=19.4326&lng=-99.1332&limite=50"

// favoriteFlags es `es_favorito` por slug; presente=false si la tarjeta no trae el campo.
func favoriteFlags(t *testing.T, cards []any) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, item := range cards {
		card := obj(t, item)
		v, ok := card["es_favorito"]
		if !ok {
			v = "omitido"
		}
		out[str(t, card["slug"])] = v
	}
	return out
}

// Integration/nv-92: con un lugar guardado y otro no, las tres listas dicen true y false;
// sin sesión el campo no va y otra persona no ve los favoritos de la primera.
func TestFavoritosTarjetas_EsFavoritoEnInicioRecomendacionesYLugares(t *testing.T) {
	env := inicioEnv(t)
	app := env.App()
	me := env.CreateUser(t, "tarjetas-fav@wherever.test")
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-tarjetas-fav", middleware.RoleConsumer)
	other := env.CreateUser(t, "tarjetas-otro@wherever.test")
	otherToken := testutil.ValidTokenFor(t, other.ID.Hex(), "jti-tarjetas-otro", middleware.RoleConsumer)

	saved := cardID(t, recoGet(t, app, recoPath, token), "cafe-ideal")
	favCall(t, app, http.MethodPut, favPath+"/negocio/"+saved, token, http.StatusOK)

	lists := map[string]func(token string) []any{
		"recomendaciones": func(tk string) []any { return list(t, getBody(t, app, recoPath, tk)["items"]) },
		"inicio":          func(tk string) []any { return list(t, getBody(t, app, inicioPath+"&hora=12:00", tk)["recomendaciones"]) },
		"lugares":         func(tk string) []any { return list(t, getBody(t, app, lugaresCercaPath, tk)["items"]) },
	}
	for name, get := range lists {
		flags := favoriteFlags(t, get(token))
		require.Contains(t, flags, "cafe-ideal", name)
		require.Greater(t, len(flags), 1, name)
		for slug, v := range flags {
			assert.Equal(t, slug == "cafe-ideal", v, "%s: %s", name, slug)
		}
		for slug, v := range favoriteFlags(t, get(otherToken)) {
			assert.Equal(t, false, v, "%s: %s no es favorito de la otra persona", name, slug)
		}
	}

	for name, cards := range map[string][]any{
		"inicio":  list(t, getBody(t, app, inicioPath+"&hora=12:00", "")["recomendaciones"]),
		"lugares": list(t, getBody(t, app, lugaresCercaPath, "")["items"]),
	} {
		require.NotEmpty(t, cards, name)
		for slug, v := range favoriteFlags(t, cards) {
			assert.Equal(t, "omitido", v, "%s sin sesión: %s", name, slug)
		}
	}
}
