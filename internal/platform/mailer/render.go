package mailer

import (
	"bytes"
	"embed"
	htmltemplate "html/template"
	texttemplate "text/template" // nosemgrep: go.lang.security.audit.xss.import-text-template.import-text-template -- solo la parte text/plain del correo; el HTML usa html/template
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Asuntos en español por tipo de correo.
var subjects = map[Kind]string{ // #nosec G101 -- asuntos de correo, no credenciales
	KindVerifyEmail:   "Confirma tu correo en Wherever",
	KindPasswordReset: "Cambia tu contraseña de Wherever",
	KindExport:        "Tu copia de datos de Wherever está lista",
}

var (
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(templateFS, "templates/*.html.tmpl"))
	textTemplates = texttemplate.Must(texttemplate.ParseFS(templateFS, "templates/*.txt.tmpl"))
)

// Rendered es el correo listo para armar el MIME.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

type templateData struct {
	Name string
	Link string
}

// Render arma asunto, texto y HTML. El HTML usa html/template: el nombre y el enlace salen
// escapados aunque traigan `<script>` o comillas.
func Render(msg Message, link string) (Rendered, error) {
	subject, ok := subjects[msg.Kind]
	if !ok {
		return Rendered{}, errUnknownKind
	}
	data := templateData{Name: msg.Name, Link: link}
	var text, html bytes.Buffer
	if err := textTemplates.ExecuteTemplate(&text, string(msg.Kind)+".txt.tmpl", data); err != nil {
		return Rendered{}, err
	}
	if err := htmlTemplates.ExecuteTemplate(&html, string(msg.Kind)+".html.tmpl", data); err != nil {
		return Rendered{}, err
	}
	return Rendered{Subject: subject, Text: text.String(), HTML: html.String()}, nil
}
