package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/sincronizacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

const syncPath = "/api/v1/sync/cambios"

func syncCall(t *testing.T, app *fiber.App, token, desde string, limit int, want int) sincronizacion.ChangesResponse {
	t.Helper()
	q := url.Values{}
	if desde != "" {
		q.Set("desde", desde)
	}
	if limit > 0 {
		q.Set("limite", fmt.Sprint(limit))
	}
	resp := sendJSON(t, app, http.MethodGet, syncPath+"?"+q.Encode(), nil, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var out sincronizacion.ChangesResponse
	if want == http.StatusOK {
		require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
		assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	}
	return out
}

func planIDs(ps []planes.Plan) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID.Hex())
	}
	return out
}

func TestSync_CompletaEIncrementalConBorrados(t *testing.T) {
	s := newGrupos(t)
	second := s.newPlan(t, s.org)

	full := syncCall(t, s.app, s.org.token, "", 0, http.StatusOK)
	assert.ElementsMatch(t, []string{s.plan.ID.Hex(), second.ID.Hex()}, planIDs(full.Plans))
	require.NotNil(t, full.Settings, "la descarga completa trae los ajustes")
	assert.False(t, full.More)
	require.NotEmpty(t, full.Next)

	// Sin cambios: nada que bajar y sin ajustes.
	idle := syncCall(t, s.app, s.org.token, full.Next, 0, http.StatusOK)
	assert.Empty(t, idle.Plans)
	assert.Empty(t, idle.Deleted)
	assert.Nil(t, idle.Settings)

	// Editar uno, borrar otro y crear un tercero.
	require.Equal(t, http.StatusOK, planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Renombrado"}, s.org.token, `"1"`).status)
	require.Equal(t, http.StatusNoContent, planCall(t, s.app, http.MethodDelete, planPath(second, ""), nil, s.org.token, `"1"`).status)
	third := s.newPlan(t, s.org)
	inc := syncCall(t, s.app, s.org.token, full.Next, 0, http.StatusOK)
	assert.Equal(t, []string{s.plan.ID.Hex(), third.ID.Hex()}, planIDs(inc.Plans), "en orden de cambio")
	assert.Equal(t, "Renombrado", inc.Plans[0].Nombre)
	require.Len(t, inc.Deleted, 1)
	assert.Equal(t, sincronizacion.DeletedItem{Kind: planes.KindPlan, ID: second.ID.Hex()}, inc.Deleted[0])
	assert.Nil(t, inc.Settings)

	// Cambiar la privacidad trae los ajustes.
	resp := sendJSON(t, s.app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]bool{"ubicacion": true}, s.org.token)
	require.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
	withSettings := syncCall(t, s.app, s.org.token, inc.Next, 0, http.StatusOK)
	require.NotNil(t, withSettings.Settings)
	assert.True(t, withSettings.Settings.Privacy.Location)
}

func TestSync_PaginaConHayMas(t *testing.T) {
	s := newGrupos(t)
	s.newPlan(t, s.org)
	s.newPlan(t, s.org)
	seen := map[string]bool{}
	next, pages := "", 0
	for {
		page := syncCall(t, s.app, s.org.token, next, 1, http.StatusOK)
		for _, id := range planIDs(page.Plans) {
			assert.False(t, seen[id], "sin repetidos")
			seen[id] = true
		}
		next = page.Next
		pages++
		if !page.More {
			break
		}
		require.Less(t, pages, 10)
	}
	assert.Len(t, seen, 3)
}

func TestSync_CursorDeHaceMasDe30DiasPideDescargaCompleta(t *testing.T) {
	s := newGrupos(t)
	signer, err := pagination.NewSigner([]byte(testutil.CursorKey))
	require.NoError(t, err)
	old := time.Now().Add(-31 * 24 * time.Hour).Unix()
	raw, err := signer.Encode(pagination.Cursor{
		ID: s.org.id(), Scope: signer.Scope("sync", s.org.id()), Sort: fmt.Sprintf("0:1:%d", old),
	})
	require.NoError(t, err)
	resp := sendJSON(t, s.app, http.MethodGet, syncPath+"?desde="+raw, nil, s.org.token)
	body := readAll(t, resp)
	require.Equal(t, http.StatusGone, resp.StatusCode, body)
	assert.Contains(t, body, "requiere_completa")
}

func TestSync_SoloLoPropio(t *testing.T) {
	s := newGrupos(t)
	other := seedAmigo(t, s.env, "sync_otro")
	theirs := syncCall(t, s.app, other.token, "", 0, http.StatusOK)
	assert.Empty(t, theirs.Plans, "nunca baja planes de otra persona")
	// Ni siendo integrante del grupo: la sincronización es de lo propio.
	befriend(t, s.env, s.org, other)
	s.create(t, []amigo{other})
	assert.Empty(t, syncCall(t, s.app, other.token, "", 0, http.StatusOK).Plans)
	// El cursor de otra persona no sirve (422).
	mine := syncCall(t, s.app, s.org.token, "", 0, http.StatusOK)
	syncCall(t, s.app, other.token, mine.Next, 0, http.StatusUnprocessableEntity)
}
