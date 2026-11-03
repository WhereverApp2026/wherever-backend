package mailer

import (
	"errors"
	"net/url"
	"strings"
)

// Rutas de las páginas que abren los enlaces (spec nv-06 §Contratos).
var linkPaths = map[Kind]string{ // #nosec G101 -- rutas de páginas, no credenciales
	KindVerifyEmail:   "/verificar",
	KindPasswordReset: "/restablecer",
	KindExport:        "/exportacion",
}

var errUnknownKind = errors.New("tipo de correo desconocido")

// Links arma los enlaces de los correos con APP_LINK_BASE.
type Links struct {
	base string
}

// NewLinks recibe APP_LINK_BASE ya validado (http(s), sin consulta ni fragmento).
func NewLinks(base string) Links {
	return Links{base: strings.TrimRight(base, "/")}
}

// URL devuelve `{base}/{ruta}#token=…`. El token va en el fragmento: el navegador no lo manda
// al servidor que sirve la página, así que no queda en sus logs ni en el Referer.
func (l Links) URL(kind Kind, token string) (string, error) {
	path, ok := linkPaths[kind]
	if !ok {
		return "", errUnknownKind
	}
	return l.base + path + "#token=" + url.QueryEscape(token), nil
}
