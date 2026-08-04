package argo

import "errors"

// ErrGitopsManaged is returned when a mutation targets a resource whose
// lifecycle belongs to its GitOps repository rather than to the API.
var ErrGitopsManaged = errors.New("cannot update gitops resources")
