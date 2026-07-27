# Utils

Shared utility module providing common helper packages used across Superphenix services.

## Packages

### `decoder`

JSON request body decoder with strict validation (disallows unknown fields, enforces single JSON object, size limits).

- `DecodeJSONBody(w, r, dst)` — Decodes an HTTP request body into a struct with comprehensive error handling.

### `error`

Standardized HTTP error response helpers.

- `HttpError(w, r, status, message, context)` — Sends a structured JSON error response with request ID, status, message, and optional context.
- `DecodeError(w, r, err)` — Translates a `decoder` error into the appropriate HTTP error response.

### `log`

Logging configuration using [zerolog](https://github.com/rs/zerolog).

- `InitLogger(level, format)` — Initializes the global logger with the specified level (`debug`, `info`, `warn`, `error`) and format (`json` or `console`).

### `validation`

Request validation utilities built on [go-playground/validator](https://github.com/go-playground/validator).

- `ValidateHTTPPayload(w, r, payload)` — Validates a struct and returns structured field-level errors as an HTTP response.
- Custom validators: `notblank`, `rfc1123`, and other domain-specific rules.
- `InvalidFieldError` / `InvalidFieldErrorV1` — Structured error types exposing field, tag, kind, and message details.
