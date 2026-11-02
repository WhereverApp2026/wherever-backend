package httpapi

import (
	_ "embed"

	"github.com/gofiber/fiber/v2"
)

// openAPISpec es el único documento OpenAPI v1; solo se sirve en dev.
//
//go:embed openapi.json
var openAPISpec []byte

// La versión de Swagger UI va fijada; la CSP de /docs en dev solo admite ese CDN.
const docsPage = `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>Wherever API v1</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>window.ui = SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" });</script>
</body>
</html>`

func registerDocs(app *fiber.App) {
	app.Get("/openapi.json", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(openAPISpec)
	})
	app.Get("/docs", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.SendString(docsPage)
	})
}
