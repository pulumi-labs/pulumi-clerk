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
	"os"
	"strings"
	"testing"
)

// The YAML runtime parses a JSON-looking config value into an object even when the config key is
// declared as a string, so wiring `claimsJson` straight into the string property `claims` fails the
// YAML type check at `pulumi preview` (QA found it, #1567). The template must go through fn::toJSON.
func TestYamlTemplateClaimsGoThroughToJSON(t *testing.T) {
	raw, err := os.ReadFile("../../../templates/clerk-cloud-yaml/Pulumi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "claims: ${claimsJson}") {
		t.Fatal("claims must not take the claimsJson config value directly: use fn::toJSON")
	}
	if !strings.Contains(text, "fn::toJSON: ${claimsJson}") {
		t.Fatal("claims must be fn::toJSON of the claimsJson config value")
	}
}
