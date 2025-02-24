// Copyright 2021 - 2025 Crunchy Data Solutions, Inc.
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// ---
// PostgreSQL identifiers are limited in length but may contain any character.
// - https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
type PostgresIdentifier = string

type PostgresPasswordSpec struct {
	// Type of password to generate. Defaults to ASCII. Valid options are ASCII
	// and AlphaNumeric.
	// "ASCII" passwords contain letters, numbers, and symbols from the US-ASCII character set.
	// "AlphaNumeric" passwords contain letters and numbers from the US-ASCII character set.
	// ---
	// Kubernetes assumes the evaluation cost of an enum value is very large.
	// TODO(k8s-1.29): Drop MaxLength after Kubernetes 1.29; https://issue.k8s.io/119511
	// +kubebuilder:validation:MaxLength=15
	//
	// +kubebuilder:default=ASCII
	// +kubebuilder:validation:Enum={ASCII,AlphaNumeric}
	// +required
	Type string `json:"type"`
}

// PostgresPasswordSpec types.
const (
	PostgresPasswordTypeAlphaNumeric = "AlphaNumeric"
	PostgresPasswordTypeASCII        = "ASCII"
)

type PostgresUserSpec struct {
	// The name of this PostgreSQL user. The value may contain only lowercase
	// letters, numbers, and hyphen so that it fits into Kubernetes metadata.
	// ---
	// This value goes into the name of a corev1.Secret and a label value, so
	// it must match both IsDNS1123Subdomain and IsValidLabelValue.
	// - https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsDNS1123Subdomain
	// - https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsValidLabelValue
	//
	// This is IsDNS1123Subdomain without any dots, U+002E:
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	//
	// +required
	Name PostgresIdentifier `json:"name"`

	// Databases to which this user can connect and create objects. Removing a
	// database from this list does NOT revoke access. This field is ignored for
	// the "postgres" user.
	// ---
	// +listType=set
	// +optional
	Databases []PostgresIdentifier `json:"databases,omitempty"`

	// ALTER ROLE options except for PASSWORD. This field is ignored for the
	// "postgres" user.
	// More info: https://www.postgresql.org/docs/current/role-attributes.html
	// ---
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^[^;]*$`
	// +kubebuilder:validation:XValidation:rule=`!self.matches("(?i:PASSWORD)")`,message="cannot assign password"
	// +kubebuilder:validation:XValidation:rule=`!self.matches("(?:--|/[*]|[*]/)")`,message="cannot contain comments"
	// +optional
	Options string `json:"options,omitempty"`

	// Properties of the password generated for this user.
	// ---
	// +optional
	Password *PostgresPasswordSpec `json:"password,omitempty"`
}
