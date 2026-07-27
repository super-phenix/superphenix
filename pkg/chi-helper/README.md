# Chi Helper

Helper functions and response rendering utilities for [go-chi/chi](https://github.com/go-chi/chi).

## Packages

### `chi-helper` (root)

HTTP response rendering functions for Chi handlers.

- MIME type constants (`MIMEJSON`, `MIMEHTML`, `MIMEXML`, `MIMEPlain`, etc.)
- `Render(w, code, r)` — Writes an HTTP response using the provided `Render` implementation.
- `RenderJSON(w, code, obj)` — Serializes an object as JSON and writes it to the response.
- `RenderString(w, code, format, values...)` — Writes a formatted string response.
- `RenderData(w, code, contentType, data)` — Writes raw bytes with a custom content type.
- `ShouldBindJSON(r, obj)` — Binds a JSON request body to a struct.
- `GetQuery(r, key)` — Retrieves a query parameter value from the request URL.

### `chi-helper/model`

Response rendering interfaces and types.

- `Render` — Interface for custom response renderers (`Render`, `WriteContentType`).
- `String` — Formatted string renderer.
- `Data` — Raw byte renderer with custom content type.
- `Status(w, code)` — Sets the HTTP status code.
- `StringToBytes` / `BytesToString` — Zero-allocation string↔byte conversions.
