package log

import (
	"context"

	middleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	ProcessIdKey = "process-id"
)

// GetLogger return logger depending on the context
func GetLogger(ctx context.Context) zerolog.Logger {
	loggedUser := ctx.Value("UserId")

	if loggedUser == nil {
		loggedUser = "unauthenticated"
	}
	return log.Logger.With().
		Ctx(ctx).
		Str("userId", loggedUser.(string)).
		Str("requestId", middleware.GetReqID(ctx)).
		Logger()
}

// GetProcessLogger return logger depending on the context - Use for Garbage collection process
// If a process-id exist, then return a dedicated logger
// Else return the default logger
func GetProcessLogger(ctx context.Context) zerolog.Logger {
	processId := ctx.Value(ProcessIdKey)

	if processId != nil {
		l := log.Logger.With().Ctx(ctx).Str("process_id", processId.(string)).Logger()
		return l
	}
	return log.Logger
}
