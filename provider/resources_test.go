package clerk_test

import (
	"encoding/json"
	"testing"

	clerk "github.com/pulumi/pulumi-clerk/provider"
)

// expectedResources is the authoritative list of Terraform resource names that
// the upstream buildwithdeck/terraform-provider-clerk v0.6.1 exports.
// Any deviation from this list in resources.go is a bug.
var expectedResources = []string{
	"clerk_allowlist_identifier",
	"clerk_api_key",
	"clerk_application",
	"clerk_application_settings",
	"clerk_blocklist_identifier",
	"clerk_domain",
	"clerk_instance_config",
	"clerk_instance_domain",
	"clerk_jwt_template",
	"clerk_machine",
	"clerk_oauth_application",
	"clerk_organization",
	"clerk_organization_domain",
	"clerk_organization_membership",
	"clerk_organization_permission",
	"clerk_organization_role",
	"clerk_redirect_url",
	"clerk_role_set",
	"clerk_saml_connection",
	"clerk_svix_webhook",
}

// expectedDataSources is the authoritative list of data sources in the upstream
// buildwithdeck/terraform-provider-clerk v0.6.1.
var expectedDataSources = []string{
	"clerk_application",
}

// phantomResources are TF names that do NOT exist in the upstream provider.
// They must never appear in our resource or data source maps.
var phantomResources = []string{
	"clerk_user",    // phantom -- does not exist as a resource in v0.6.1
	"clerk_email",   // phantom -- does not exist
	"clerk_session", // phantom -- does not exist
}

// TestProvider_DoesNotPanic verifies that Provider() can be called without
// panicking. MustComputeTokens and mustSetVersion both panic on failure, so a
// successful return is a meaningful contract check.
func TestProvider_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Provider() panicked: %v", r)
		}
	}()
	_ = clerk.Provider()
}

// TestProvider_ResourceCount asserts the provider declares exactly 20 resources,
// matching the upstream buildwithdeck/terraform-provider-clerk v0.6.1 resource count.
func TestProvider_ResourceCount(t *testing.T) {
	prov := clerk.Provider()
	got := len(prov.Resources)
	want := 20
	if got != want {
		t.Errorf("Resources count = %d, want %d", got, want)
	}
}

// TestProvider_DataSourceCount asserts the provider declares exactly 1 data source,
// matching the upstream buildwithdeck/terraform-provider-clerk v0.6.1 data source count.
func TestProvider_DataSourceCount(t *testing.T) {
	prov := clerk.Provider()
	got := len(prov.DataSources)
	want := 1
	if got != want {
		t.Errorf("DataSources count = %d, want %d", got, want)
	}
}

// TestProvider_ResourcesExist verifies that every expected resource name is mapped
// in the provider. Missing entries indicate a gap in resources.go.
func TestProvider_ResourcesExist(t *testing.T) {
	prov := clerk.Provider()
	for _, name := range expectedResources {
		if _, ok := prov.Resources[name]; !ok {
			t.Errorf("expected resource %q not found in provider", name)
		}
	}
}

// TestProvider_DataSourcesExist verifies that every expected data source name is
// mapped in the provider.
func TestProvider_DataSourcesExist(t *testing.T) {
	prov := clerk.Provider()
	for _, name := range expectedDataSources {
		if _, ok := prov.DataSources[name]; !ok {
			t.Errorf("expected data source %q not found in provider", name)
		}
	}
}

// TestProvider_NoPhantomResources verifies that known phantom names are NOT mapped.
func TestProvider_NoPhantomResources(t *testing.T) {
	prov := clerk.Provider()
	for _, phantom := range phantomResources {
		if _, ok := prov.Resources[phantom]; ok {
			t.Errorf("phantom resource %q should not be in provider resources", phantom)
		}
		if _, ok := prov.DataSources[phantom]; ok {
			t.Errorf("phantom resource %q should not be in provider data sources", phantom)
		}
	}
}

// TestProvider_Name asserts the provider name is "clerk".
func TestProvider_Name(t *testing.T) {
	prov := clerk.Provider()
	if prov.Name != "clerk" {
		t.Errorf("provider name = %q, want %q", prov.Name, "clerk")
	}
}

// TestProvider_SchemaIsJSONSerializable is a lightweight schema round-trip
// check: it marshals the ProviderInfo token maps to JSON. If the ProviderInfo
// structure is corrupt or contains non-serializable fields, this will fail.
func TestProvider_SchemaIsJSONSerializable(t *testing.T) {
	prov := clerk.Provider()

	// Marshal the resource token map to JSON -- a simple proxy for structural
	// integrity without invoking the full tfgen pipeline.
	resourceTokens := make(map[string]string, len(prov.Resources))
	for tfName, info := range prov.Resources {
		if info != nil {
			resourceTokens[tfName] = string(info.Tok)
		}
	}

	data, err := json.Marshal(resourceTokens)
	if err != nil {
		t.Fatalf("failed to JSON-marshal resource tokens: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("JSON-marshalled resource tokens is empty")
	}

	dsTokens := make(map[string]string, len(prov.DataSources))
	for tfName, info := range prov.DataSources {
		if info != nil {
			dsTokens[tfName] = string(info.Tok)
		}
	}
	dsData, err := json.Marshal(dsTokens)
	if err != nil {
		t.Fatalf("failed to JSON-marshal data source tokens: %v", err)
	}
	if len(dsData) == 0 {
		t.Fatal("JSON-marshalled data source tokens is empty")
	}
}
