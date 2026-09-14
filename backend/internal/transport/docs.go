package transport

import (
	"embed"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed swaggerui/index.html swaggerui/swagger-initializer.js swaggerui/swagger-ui.css swaggerui/swagger-ui-bundle.js swaggerui/favicon.ico swaggerui/logo-mark.png swaggerui/LICENSE swaggerui/swagger-ui-bundle.js.LICENSE.txt
var docsFiles embed.FS

const docsCSP = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"

func registerDocs(engine *gin.Engine, document []byte) error {
	// Register exact paths, so no directory, source map or filesystem path is exposed.
	for _, asset := range []struct{ route, name, contentType string }{
		{"/docs/", "index.html", "text/html; charset=utf-8"},
		{"/docs/swagger-initializer.js", "swagger-initializer.js", "application/javascript; charset=utf-8"},
		{"/docs/swagger-ui.css", "swagger-ui.css", "text/css; charset=utf-8"},
		{"/docs/swagger-ui-bundle.js", "swagger-ui-bundle.js", "application/javascript; charset=utf-8"},
		{"/docs/favicon.ico", "favicon.ico", "image/x-icon"},
		{"/docs/logo-mark.png", "logo-mark.png", "image/png"},
		{"/docs/LICENSE", "LICENSE", "text/plain; charset=utf-8"},
		{"/docs/swagger-ui-bundle.js.LICENSE.txt", "swagger-ui-bundle.js.LICENSE.txt", "text/plain; charset=utf-8"},
	} {
		data, err := docsFiles.ReadFile("swaggerui/" + asset.name)
		if err != nil {
			return errors.New("embedded API documentation is unavailable")
		}
		engine.GET(asset.route, documentationResponse(asset.contentType, data))
	}
	engine.GET("/openapi.yaml", documentationResponse("application/yaml; charset=utf-8", document))
	return nil
}

func documentationResponse(contentType string, data []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", docsCSP)
		c.Header("Referrer-Policy", "no-referrer")
		c.Data(http.StatusOK, contentType, data)
	}
}
