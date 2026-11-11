// Copyright 2026, Pulumi Corporation.
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

package provider

// Clerk's server-side defaults for optional inputs. On import an optional input
// is adopted only when the live value differs from these: adopting a default
// would plan an update for a program that leaves the input unset (#1563), and
// adopting nothing would plan one for a program that declares a real value
// (#1801).
const (
	clerkDefaultMachineTokenTTL       = 3600
	clerkDefaultJwtLifetime           = 60
	clerkDefaultJwtClockSkew          = 5
	clerkDefaultJwtCustomSigningKey   = false
	clerkDefaultJwtSigningAlgorithm   = "RS256"
	clerkDefaultMaxAllowedMemberships = 5
	clerkDefaultAdminDeleteEnabled    = true
	clerkDefaultConsentScreenEnabled  = false
	clerkDefaultDomainsEnabled        = false
)

// adoptIfNotDefault returns a pointer to live when it differs from def, and nil
// (leave the input unset) when it equals the default.
func adoptIfNotDefault[T comparable](live, def T) *T {
	if live == def {
		return nil
	}
	return &live
}
