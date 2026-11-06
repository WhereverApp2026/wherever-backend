package httpapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Orden constitucional §4; el critic (B3) revisa la misma lista en el código fuente.
func TestPipeline_ConstitutionalOrder(t *testing.T) {
	log, _ := testutil.Logger()
	deps := httpapi.Deps{Logger: log, Redis: testutil.DeadRedis(t)}

	var names []string
	for _, s := range httpapi.Pipeline(testutil.Config("dev"), deps, middleware.NewLimiter(deps.Redis, log)) {
		names = append(names, s.Name)
	}

	assert.Equal(t, []string{
		"security_headers", "errors", "request_id", "request_timeout", "proxy_headers",
		"cors", "compress", "rate_limit_global", "rate_limit_writes", "jwt",
	}, names)
}
