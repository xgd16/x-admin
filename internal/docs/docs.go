// Package docs 内嵌 OpenAPI 文档与 Swagger UI 页面。
package docs

import _ "embed"

// OpenAPI 是构建时嵌入的 OpenAPI 3 描述文件（openapi.yaml）。
//
//go:embed openapi.yaml
var OpenAPI []byte

// IndexHTML 是 Swagger UI 页面，资源通过 CDN 加载，因此部署环境需要外网访问。
const IndexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>x-admin · API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #edf3f9; }
    .topbar { display: none; }
    .swagger-ui .info .title { color: #0b1c2c; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/swagger/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout'
    })
  </script>
</body>
</html>`
