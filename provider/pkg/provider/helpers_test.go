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

import (
	"testing"

	"github.com/blang/semver"
	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

// These helpers drive resources through the real provider RPC surface against
// fakeClerk (see fake_clerk_test.go).

func pm(kv map[string]property.Value) property.Map { return property.NewMap(kv) }

func str(s string) property.Value { return property.New(s) }

func num(n int) property.Value { return property.New(float64(n)) }

func boolean(b bool) property.Value { return property.New(b) }

func strArray(vals ...string) property.Value {
	arr := make([]property.Value, 0, len(vals))
	for _, v := range vals {
		arr = append(arr, property.New(v))
	}
	return property.New(property.NewArray(arr))
}

func clerkUrn(typ, name string) resource.URN {
	return resource.NewURN("stack", "proj", "", tokens.Type("clerk:index:"+typ), name)
}

// serverFor returns a provider server configured against the fake.
func serverFor(t *testing.T, f *fakeClerk) integration.Server {
	t.Helper()
	for _, v := range []string{"CLERK_API_KEY", "CLERK_SECRET_KEY", "CLERK_PLATFORM_API_KEY", "CLERK_API_URL"} {
		t.Setenv(v, "")
	}
	p, err := New()
	require.NoError(t, err)
	s, err := integration.NewServer(t.Context(), "clerk", semver.MustParse("0.0.1"), integration.WithProvider(p))
	require.NoError(t, err)
	cfg := map[string]property.Value{
		"apiKey":         str(fakeAPIKey),
		"platformApiKey": str(fakePlatformAPIKey),
		"apiUrl":         str(f.server.URL),
	}
	require.NoError(t, s.Configure(pgp.ConfigureRequest{Args: pm(cfg)}))
	return s
}

func getStr(t *testing.T, m property.Map, key string) string {
	t.Helper()
	v, ok := m.GetOk(key)
	require.True(t, ok, "property %q missing from %v", key, m)
	return v.AsString()
}

func requireNoChanges(t *testing.T, d pgp.DiffResponse) {
	t.Helper()
	require.False(t, d.HasChanges, "unexpected diff: %+v", d.DetailedDiff)
}

func invoke(t *testing.T, s integration.Server, token string, args map[string]property.Value) (property.Map, error) {
	t.Helper()
	resp, err := s.Invoke(pgp.InvokeRequest{Token: tokens.Type("clerk:index:" + token), Args: pm(args)})
	if err != nil {
		return property.Map{}, err
	}
	return resp.Return, nil
}
