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
	"k8s.io/client-go/util/retry"
)

// ErrBelowUsage is returned when a limit would be lowered below what the project currently uses.
var ErrBelowUsage = errors.New("quota below what is currently used")

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
// When revision is given and the existing ResourceQuota has a newer revision annotation,
// the update is skipped (stale push).
//
// A limit that is lowered cannot go below its `status.used` (ErrBelowUsage), so that capacity
// still in use is never freed in the API for another AZ or project. The check and the update
// are atomic: the quota admission updates `status.used` on the same object, so a workload
// admitted in between makes the update conflict, and the check is run again.
func Apply(ctx context.Context, orgId, projectId string, hard v1.ResourceList, revision ...int64) error {
	var rev int64
	if len(revision) > 0 {
		rev = revision[0]
	}
	if err := k8s.CreateNamespaceIfNotExists(ctx, orgId, projectId); err != nil {
		return err
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return apply(ctx, orgId, projectId, hard, rev)
	})
}

func apply(ctx context.Context, orgId, projectId string, hard v1.ResourceList, revision int64) error {
	m := spxId.Metadata{OrgId: orgId, ProjectId: projectId}
	namespace := utils.GetNamespace(projectId)
	client := config.K8sClient.CoreV1().ResourceQuotas(namespace)

	name := GetName(projectId)
	current, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		annotations := map[string]string{}
		if revision > 0 {
			annotations[spxId.SpxAnnotationQuotaRevision] = strconv.FormatInt(revision, 10)
		}
		_, err = client.Create(ctx, &v1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Labels:      m.GetLabels(),
				Annotations: annotations,
			},
			Spec: v1.ResourceQuotaSpec{Hard: hard},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if IsGitops(current) {
		log := logger.GetLogger(ctx)
		log.Info().Str("namespace", namespace).Str("name", name).Msg("ResourceQuota is managed by GitOps, skipping reconciliation")
		return nil
	}

	if revision > 0 && current.Annotations != nil {
		if storedRevStr, ok := current.Annotations[spxId.SpxAnnotationQuotaRevision]; ok {
			if storedRev, parseErr := strconv.ParseInt(storedRevStr, 10, 64); parseErr == nil {
				if storedRev > revision {
					log := logger.GetLogger(ctx)
					log.Info().
						Str("namespace", namespace).
						Str("name", name).
						Int64("storedRevision", storedRev).
						Int64("incomingRevision", revision).
						Msg("ResourceQuota has newer revision, skipping stale update")
					return nil
				}
			}
		}
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
	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	if revision > 0 {
		current.Annotations[spxId.SpxAnnotationQuotaRevision] = strconv.FormatInt(revision, 10)
	}
	current.Spec.Hard = hard
	_, err = client.Update(ctx, current, metav1.UpdateOptions{})
	return err
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

// Delete removes the ResourceQuota of the project, if any. A GitOps-managed ResourceQuota is
// skipped: it is left as is and no error is returned. When revision is given and the existing
// ResourceQuota has a newer revision annotation, the deletion is skipped (stale deletion).
func Delete(ctx context.Context, projectId string, revision ...int64) error {
	var rev int64
	if len(revision) > 0 {
		rev = revision[0]
	}
	namespace := utils.GetNamespace(projectId)
	name := GetName(projectId)
	client := config.K8sClient.CoreV1().ResourceQuotas(namespace)

	current, err := client.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if IsGitops(current) {
		log := logger.GetLogger(ctx)
		log.Info().Str("namespace", namespace).Str("name", name).Msg("ResourceQuota is managed by GitOps, skipping deletion")
		return nil
	}

	if rev > 0 && current.Annotations != nil {
		if storedRevStr, ok := current.Annotations[spxId.SpxAnnotationQuotaRevision]; ok {
			if storedRev, parseErr := strconv.ParseInt(storedRevStr, 10, 64); parseErr == nil {
				if storedRev > rev {
					log := logger.GetLogger(ctx)
					log.Info().
						Str("namespace", namespace).
						Str("name", name).
						Int64("storedRevision", storedRev).
						Int64("incomingRevision", rev).
						Msg("ResourceQuota has newer revision, skipping stale deletion")
					return nil
				}
			}
		}
	}

	err = client.Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
