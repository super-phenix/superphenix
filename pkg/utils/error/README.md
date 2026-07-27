# HTTP Error Package

A lightweight, fluent API for sending structured JSON error responses in HTTP handlers. Inspired by [zerolog](https://github.com/rs/zerolog)'s chaining pattern.

## Response Format

All errors are returned as JSON with the following structure:

```json
{
  "message": "human-readable error description",
  "context": {
    "requestId": "middleware/request-id-value",
    "key": "additional context value"
  }
}
```

The `requestId` field is automatically injected from Chi's `RequestID` middleware when available.

## API Reference

### Creating an Error Response

```go
import httpError "github.com/super-phenix/superphenix/pkg/utils/error"

// Create an error response with an HTTP status code
httpError.Http(w, r, http.StatusBadRequest)
```

`Http(w, r, code)` sets the `Content-Type` to `application/json`, writes the HTTP status code, attaches the request ID from context, and returns a `*Message` for chaining.

### Sending the Error

| Method                    | Description                          | Example                                             |
|---------------------------|--------------------------------------|-----------------------------------------------------|
| `.Msg(string)`            | Send with a message                  | `.Msg("resource not found")`                        |
| `.Msgf(string, ...any)`   | Send with a formatted message        | `.Msgf("invalid ID: %s", id)`                       |
| `.MsgFunc(func() string)` | Send with a lazily-evaluated message | `.MsgFunc(func() string { return expensiveMsg() })` |
| `.Send()`                 | Send with an empty message           | `.Send()`                                           |

> **⚠️ Important:** Once any send method is called, the `*Message` must be disposed. Calling a send method twice produces unexpected results.

### Adding Context

```go
httpError.Http(w, r, http.StatusConflict).
    Str("resourceId", id).
    Any("details", detailsObj).
    Msg("resource already exists")
```

| Method           | Description                                    |
|------------------|------------------------------------------------|
| `.Str(key, val)` | Add a string key-value pair to the context     |
| `.Any(key, val)` | Add any JSON-serializable value to the context |

All context methods return `*Message` for chaining.

### Decoding Upstream Errors

When proxying requests to other services, use `RetrieveHttpError` to decode error responses:

```go
import httpError "github.com/super-phenix/superphenix/pkg/utils/error"

errBody := httpError.RetrieveHttpError(resp)
if errBody != nil {
    // errBody.Message contains the upstream error message
    // errBody.Context contains the upstream context map
}
```

## Usage Patterns

### Standard Handler Error

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    log := logger.GetLogger(r.Context())

    result, err := doSomething()
    if err != nil {
        log.Error().Err(err).Msg("failed to do something")
        httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to do something")
        return
    }
    // ... success path
}
```

### With Additional Context

```go
if err != nil {
    log.Error().Err(err).Str("projectId", projectId).Msg("project not found")
    httpError.Http(w, r, http.StatusNotFound).
        Str("projectId", projectId).
        Msg("project not found")
    return
}
```

### Forwarding Upstream Errors

```go
resp, err := proxy.SendRequest(ctx, url, "GET", nil, authSecret)
if err != nil {
    httpError.Http(w, r, http.StatusBadGateway).Msg("upstream service unavailable")
    return
}
if resp.StatusCode != http.StatusOK {
    if errBody := httpError.RetrieveHttpError(resp); errBody != nil {
        httpError.Http(w, r, resp.StatusCode).Msg(errBody.Message)
        return
    }
    httpError.Http(w, r, resp.StatusCode).Send()
    return
}
```

## Guidelines

1. **Always use `httpError.Http()`** — Do not use `http.Error()` or write raw responses for error cases. The custom package ensures consistent JSON format and automatic request ID injection.
2. **Always log before sending** — Log the error with `zerolog` (including the Go `error` object via `.Err(err)`) before sending the HTTP error response. The HTTP response should contain a user-safe message, not internal details.
3. **Use predefined constants** — When available, use error message and code constants from `internal/consts/` (Superphenix API) or local constants rather than inline strings.
4. **Return after sending** — Always `return` immediately after sending an error to prevent further handler execution.
5. **Nil-safe** — All methods on `*Message` are nil-safe (they check `if e == nil`), but this should not be relied upon in normal flow.
