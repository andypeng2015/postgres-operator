// Copyright 2021 - 2023 Crunchy Data Solutions, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crunchybridgecluster

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crunchydata/postgres-operator/internal/bridge"
	"github.com/crunchydata/postgres-operator/internal/initialize"
	"github.com/crunchydata/postgres-operator/internal/naming"
	"github.com/crunchydata/postgres-operator/pkg/apis/postgres-operator.crunchydata.com/v1beta1"
)

// generatePostgresRoleSecret returns a Secret containing a password and
// connection details for the appropriate database.
func (r *CrunchyBridgeClusterReconciler) generatePostgresRoleSecret(
	cluster *v1beta1.CrunchyBridgeCluster, roleSpec *v1beta1.CrunchyBridgeClusterRoleSpec,
	clusterRole *bridge.ClusterRoleApiResource,
) (*corev1.Secret, error) {
	roleName := roleSpec.Name
	secretName := roleSpec.SecretName
	intent := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace: cluster.Namespace,
		Name:      secretName,
	}}
	intent.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	initialize.StringMap(&intent.StringData)

	intent.StringData["name"] = clusterRole.Name
	intent.StringData["password"] = clusterRole.Password
	intent.StringData["uri"] = clusterRole.URI

	intent.Annotations = cluster.Spec.Metadata.GetAnnotationsOrNil()
	intent.Labels = naming.Merge(
		cluster.Spec.Metadata.GetLabelsOrNil(),
		map[string]string{
			naming.LabelCluster: cluster.Name,
			naming.LabelRole:    naming.RoleCrunchyBridgeClusterPostgresRole,
			naming.LabelCrunchyBridgeClusterPostgresRole: roleName,
		})

	err := errors.WithStack(r.setControllerReference(cluster, intent))

	return intent, err
}

// reconcilePostgresRoles writes the objects necessary to manage roles and their
// passwords in PostgreSQL.
func (r *CrunchyBridgeClusterReconciler) reconcilePostgresRoles(
	ctx context.Context, apiKey string, cluster *v1beta1.CrunchyBridgeCluster,
) error {
	_, _, err := r.reconcilePostgresRoleSecrets(ctx, apiKey, cluster)

	// TODO: If we ever add a PgAdmin feature to CrunchyBridgeCluster, we will
	// want to add the role credentials to PgAdmin here

	return err
}

func (r *CrunchyBridgeClusterReconciler) reconcilePostgresRoleSecrets(
	ctx context.Context, apiKey string, cluster *v1beta1.CrunchyBridgeCluster,
) (
	[]*v1beta1.CrunchyBridgeClusterRoleSpec, map[string]*corev1.Secret, error,
) {
	log := ctrl.LoggerFrom(ctx)
	specRoles := cluster.Spec.Roles

	// Index role specifications by PostgreSQL role name.
	roleSpecs := make(map[string]*v1beta1.CrunchyBridgeClusterRoleSpec, len(specRoles))
	for i := range specRoles {
		roleSpecs[specRoles[i].Name] = specRoles[i]
	}

	// Gather existing role secrets
	secrets := &corev1.SecretList{}
	selector, err := naming.AsSelector(naming.CrunchyBridgeClusterPostgresRoles(cluster.Name))
	if err == nil {
		err = errors.WithStack(
			r.Client.List(ctx, secrets,
				client.InNamespace(cluster.Namespace),
				client.MatchingLabelsSelector{Selector: selector},
			))
	}

	// Index secrets by PostgreSQL role name and delete any that are not in the
	// cluster spec.
	roleSecrets := make(map[string]*corev1.Secret, len(secrets.Items))
	if err == nil {
		for i := range secrets.Items {
			secret := &secrets.Items[i]
			secretRoleName := secret.Labels[naming.LabelCrunchyBridgeClusterPostgresRole]

			roleSpec, specified := roleSpecs[secretRoleName]
			if specified && roleSpec.SecretName == secret.Name {
				roleSecrets[secretRoleName] = secret
			} else if err == nil {
				err = errors.WithStack(r.deleteControlled(ctx, cluster, secret))
			}
		}
	}

	// Reconcile each PostgreSQL role in the cluster spec.
	for roleName, role := range roleSpecs {
		// Get ClusterRole from Bridge API
		clusterRole, err := r.NewClient().GetClusterRole(ctx, apiKey, cluster.Status.ID, roleName)
		// If issue with getting ClusterRole, log error and move on to next role
		if err != nil {
			// TODO (dsessler7): Emit event here?
			log.Error(err, "whoops, issue retrieving cluster role")
			continue
		}
		if err == nil {
			roleSecrets[roleName], err = r.generatePostgresRoleSecret(cluster, role, clusterRole)
		}
		if err == nil {
			err = errors.WithStack(r.apply(ctx, roleSecrets[roleName]))
		}
		if err != nil {
			log.Error(err, "Issue creating role secret.")
		}
	}

	return specRoles, roleSecrets, err
}
