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
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	pgp "github.com/pulumi/pulumi-go-provider"
	"github.com/stretchr/testify/require"
)

// An import of an ID that does not exist must fail instead of storing an empty resource
// (#1715). A refresh of an object deleted outside Pulumi (prior state present) still drops
// it; the per-resource tests cover that side.
func TestImportOfMissingObjectFails(t *testing.T) {
	f := startFakeClerk(t)
	s := serverFor(t, f)
	for typ, id := range map[string]string{
		"AllowlistIdentifier": "alid_nope",
		"BlocklistIdentifier": "blid_nope",
		"JwtTemplate":         "jtmpl_nope",
		"Machine":             "mch_nope",
		"Organization":        "org_nope",
		"RedirectUrl":         "ru_nope",
	} {
		_, err := s.Read(pgp.ReadRequest{ID: id, Urn: clerkUrn(typ, "gone"), Properties: pm(nil), Inputs: pm(nil)})
		require.ErrorContains(t, err, "does not exist", typ)
	}
}

// Every resource Read must carry the import guard, so a new resource cannot reintroduce the
// defect. ApplicationSettings is the one exception: a singleton the instance always has.
func TestEveryReadGuardsImportOfMissingObject(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)
	reads := 0
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Name.Name != "Read" || fn.Body == nil {
					continue
				}
				if fname == "application_settings.go" {
					continue
				}
				reads++
				guarded := false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "notFoundOnImport" {
							guarded = true
						}
					}
					return !guarded
				})
				require.True(t, guarded, "%s: Read must return notFoundOnImport before an empty not-found response", fname)
			}
		}
	}
	require.Positive(t, reads)
}
