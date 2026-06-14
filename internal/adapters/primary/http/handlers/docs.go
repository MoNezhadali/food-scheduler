package handlers

import (
	"net/http"

	"github.com/MoNezhadali/foodscheduler/api"
)

// DocsHandler serves the OpenAPI spec (YAML) and a ReDoc browser UI.
type DocsHandler struct{}

func NewDocsHandler() *DocsHandler { return &DocsHandler{} }

func (h *DocsHandler) ServeSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(api.SpecYAML) //nolint:errcheck
}

func (h *DocsHandler) ServeUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(redocPage)) //nolint:errcheck
}

const redocPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>FoodScheduler API Docs</title>
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url="/openapi.yaml"></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@2.1.5/bundles/redoc.standalone.js"></script>
</body>
</html>`
