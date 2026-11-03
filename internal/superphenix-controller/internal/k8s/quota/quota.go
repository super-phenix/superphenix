// Package quota enforces the quota of a project in this AZ, as computed by the
// Superphenix API, through a Kubernetes ResourceQuota in the project namespace.
package quota

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

var (
	// ErrBelowUsage is returned when a limit would be lowered below what the project currently uses.
	ErrBelowUsage = errors.New("quota below what is currently used")
	// ErrStale is returned when a push is older than the revision of the ResourceQuota. It is
	// returned as a *StaleError, which holds the stored revision.
	ErrStale = errors.New("a newer quota revision is enforced")
)

// StaleError is the ErrStale of a push older than the revision annotation of the ResourceQuota.
type StaleError struct {
	// Stored is the revision annotation of the ResourceQuota.
	Stored int64
	// Pushed is the revision of the refused push.
	Pushed int64
}

// Error reports the stored and the pushed revision.
func (e *StaleError) Error() string {
	return fmt.Sprintf("%s: %d, pushed %d", ErrStale, e.Stored, e.Pushed)
}

// Is makes errors.Is(err, ErrStale) true for a *StaleError.
func (e *StaleError) Is(target error) bool {
	return target == ErrStale
}

// conflictBackoff bounds the retries of a push whose ResourceQuota was updated concurrently, by
// another push or by the quota admission updating `status.used`. Its worst case (about 2.5s) stays
// well below the push timeout of the API.
var conflictBackoff = wait.Backoff{Steps: 8, Duration: 10 * time.Millisecond, Factor: 2, Jitter: 0.2, Cap: time.Second}

// GetName returns the name of the ResourceQuota of a project. It must stay identical to the
// name rendered by the sfs-iaas chart (templates/namespace/resourcequota.yaml), so that both
// manage the same object instead of stacking two quotas.
func GetName(projectId string) string {
	return fmt.Sprintf("quota-%s", projectId)
}

// MaxResources bounds the number of resources of a quota.
const MaxResources = 128

// Validate checks that hard only holds valid Kubernetes quota resource names and quantities.
func Validate(hard map[string]string) (v1.ResourceList, error) {
	if len(hard) > MaxResources {
		return nil, fmt.Errorf("at most %d resources are allowed", MaxResources)
	}
	list := make(v1.ResourceList, len(hard))
	for name, value := range hard {
		if errs := validation.IsQualifiedName(name); len(errs) > 0 {
			return nil, fmt.Errorf("invalid resource name %q: %s", name, strings.Join(errs, "; "))
		}
		q, err := resource.ParseQuantity(value)
		if err != nil {
			return nil, fmt.Errorf("invalid quantity %q for %s: %v", value, name, err)
		}
		if q.Sign() < 0 {
			return nil, fmt.Errorf("%s cannot be negative", name)
		}
		list[v1.ResourceName(name)] = q
	}
	return list, nil
}

// IsGitops reports whether the ResourceQuota is managed by GitOps (rendered by the sfs-iaas chart
// with gitops enabled). Like the other GitOps resources, the controller then leaves it untouched.
func IsGitops(rq *v1.ResourceQuota) bool {
	return rq.GetLabels()[spxId.SpxLabelGitops] == "true"
}

// Apply creates or replaces the ResourceQuota of the project with hard, creating the
// project namespace if needed. Resources absent from hard are no longer limited.
// A GitOps-managed ResourceQuota is skipped: it is left as is and no error is returned.
// A push older than the revision annotation of the existing ResourceQuota is refused with
// ErrStale (see checkRevision); the annotation is set when revision is not 0.
//
// A limit that is lowered cannot go below its `status.used` (ErrBelowUsage), so that capacity
// still in use is never freed in the API for another AZ or project. The check and the update
// are atomic: the quota admission updates `status.used` on the same object, so a workload
// admitted in between makes the update conflict, and the check is run again.
func Apply(ctx context.Context, orgId, projectId string, hard v1.ResourceList, revision int64) error {
	if err := k8s.CreateNamespaceIfNotExists(ctx, orgId, projectId); err != nil {
		return err
	}
	return retryOnConflict(func() error {
		return apply(ctx, orgId, projectId, hard, revision)
	})
}

// apply runs one attempt of Apply.
func apply(ctx context.Context, orgId, projectId string, hard v1.ResourceList, revision int64) error {
	m := spxId.Metadata{OrgId: orgId, ProjectId: projectId}
	namespace := utils.GetNamespace(projectId)
	client := config.K8sClient.CoreV1().ResourceQuotas(namespace)

	name := GetName(projectId)
	current, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return create(ctx, orgId, projectId, hard, revision)
	}
	if err != nil {
		return err
	}
	if IsGitops(current) {
		log := logger.GetLogger(ctx)
		log.Info().Str("namespace", namespace).Str("name", name).Msg("ResourceQuota is managed by GitOps, skipping reconciliation")
		return nil
	}

	if err := checkRevision(current, revision); err != nil {
		return err
	}

	if err := CheckNotBelowUsage(current, hard); err != nil {
		return err
	}

	// The quota may have been created by the sfs-iaas chart: its other labels are kept and the
	// SPX labels (shared with the chart) are re-applied so that "all" backups still select it.
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	for key, value := range m.GetLabels() {
		current.Labels[key] = value
	}
	setRevision(current, revision)
	current.Spec.Hard = hard
	_, err = client.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

// create creates the ResourceQuota of the project with hard, annotated with revision.
func create(ctx context.Context, orgId, projectId string, hard v1.ResourceList, revision int64) error {
	m := spxId.Metadata{OrgId: orgId, ProjectId: projectId}
	namespace := utils.GetNamespace(projectId)
	rq := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetName(projectId),
			Namespace: namespace,
			Labels:    m.GetLabels(),
		},
		Spec: v1.ResourceQuotaSpec{Hard: hard},
	}
	setRevision(rq, revision)
	_, err := config.K8sClient.CoreV1().ResourceQuotas(namespace).Create(ctx, rq, metav1.CreateOptions{})
	return err
}

// setRevision sets the revision annotation of rq, unless revision is 0.
func setRevision(rq *v1.ResourceQuota, revision int64) {
	if revision == 0 {
		return
	}
	if rq.Annotations == nil {
		rq.Annotations = map[string]string{}
	}
	rq.Annotations[spxId.SpxAnnotationQuotaRevision] = strconv.FormatInt(revision, 10)
}

// retryOnConflict runs fn again when the ResourceQuota was updated (Conflict) or created
// (AlreadyExists) concurrently.
func retryOnConflict(fn func() error) error {
	return retry.OnError(conflictBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, fn)
}

// Get returns the limits (`spec.hard`) and the usage (`status.used`) of the ResourceQuota of the
// project, GitOps-managed or not. Both are empty when the project has no ResourceQuota.
// Kubernetes only tracks the usage of the resources the quota limits.
func Get(ctx context.Context, projectId string) (hard, used v1.ResourceList, err error) {
	namespace := utils.GetNamespace(projectId)
	current, err := config.K8sClient.CoreV1().ResourceQuotas(namespace).Get(ctx, GetName(projectId), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return v1.ResourceList{}, v1.ResourceList{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	hard, used = current.Spec.Hard, current.Status.Used
	if hard == nil {
		hard = v1.ResourceList{}
	}
	if used == nil {
		used = v1.ResourceList{}
	}
	return hard, used, nil
}

// CheckNotBelowUsage checks that hard does not lower any limit of current below its
// `status.used`. Only lowered limits are checked: a limit kept as is (even below the usage, e.g.
// set before this check) or raised is accepted, and a resource not limited before has no usage.
func CheckNotBelowUsage(current *v1.ResourceQuota, hard v1.ResourceList) error {
	names := make([]string, 0, len(hard))
	for name := range hard {
		names = append(names, string(name))
	}
	slices.Sort(names)
	for _, name := range names {
		limit := hard[v1.ResourceName(name)]
		old, limited := current.Spec.Hard[v1.ResourceName(name)]
		if !limited || limit.Cmp(old) >= 0 {
			continue
		}
		if used, ok := current.Status.Used[v1.ResourceName(name)]; ok && used.Cmp(limit) > 0 {
			return fmt.Errorf("%w: %s %s used, above the requested %s", ErrBelowUsage, name, used.String(), limit.String())
		}
	}
	return nil
}

// Delete empties the `spec.hard` of the ResourceQuota of the project and keeps the object,
// annotated with revision when it is not 0. When the project has no ResourceQuota, Delete creates
// an empty one, so that checkRevision can refuse an older push later (see createEmpty). Delete
// refuses a push older than the revision annotation with ErrStale (see checkRevision) and skips a
// GitOps-managed ResourceQuota.
func Delete(ctx context.Context, orgId, projectId string, revision int64) error {
	return retryOnConflict(func() error {
		return clearLimits(ctx, orgId, projectId, revision)
	})
}

// clearLimits runs one attempt of Delete.
func clearLimits(ctx context.Context, orgId, projectId string, revision int64) error {
	namespace := utils.GetNamespace(projectId)
	name := GetName(projectId)
	client := config.K8sClient.CoreV1().ResourceQuotas(namespace)

	current, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return createEmpty(ctx, orgId, projectId, revision)
	}
	if err != nil {
		return err
	}
	if IsGitops(current) {
		log := logger.GetLogger(ctx)
		log.Info().Str("namespace", namespace).Str("name", name).Msg("ResourceQuota is managed by GitOps, skipping deletion")
		return nil
	}

	if err := checkRevision(current, revision); err != nil {
		return err
	}
	setRevision(current, revision)
	current.Spec.Hard = v1.ResourceList{}
	_, err = client.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

// createEmpty creates an empty ResourceQuota annotated with revision, so that an older push is
// refused later. Nothing is created when revision is 0, or when the project namespace is missing or
// terminating: nothing runs there.
func createEmpty(ctx context.Context, orgId, projectId string, revision int64) error {
	if revision == 0 {
		return nil
	}
	namespace, err := config.K8sClient.CoreV1().Namespaces().Get(ctx, utils.GetNamespace(projectId), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if namespace.DeletionTimestamp != nil || namespace.Status.Phase == v1.NamespaceTerminating {
		return nil
	}
	err = create(ctx, orgId, projectId, v1.ResourceList{}, revision)
	if apierrors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		return nil
	}
	return err
}

// checkRevision returns a *StaleError when revision is older than the revision annotation of
// current. A push without revision (0) is older than any annotated ResourceQuota.
func checkRevision(current *v1.ResourceQuota, revision int64) error {
	stored, err := strconv.ParseInt(current.Annotations[spxId.SpxAnnotationQuotaRevision], 10, 64)
	if err != nil || stored <= revision {
		return nil
	}
	return &StaleError{Stored: stored, Pushed: revision}
}
