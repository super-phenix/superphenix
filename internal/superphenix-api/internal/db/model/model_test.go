package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// productTypeConstants parses the ProductType* constants out of model.go.
func productTypeConstants(t *testing.T) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "model.go", nil, 0)
	require.NoError(t, err)

	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "ProductType") {
					continue
				}
				lit, ok := valueSpec.Values[i].(*ast.BasicLit)
				require.True(t, ok, "%s must be a string literal", name.Name)
				value, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				constants[name.Name] = value
			}
		}
	}

	require.NotEmpty(t, constants, "no ProductType* constant found in model.go")
	return constants
}

func TestProductTypeReadPermissionCoverage(t *testing.T) {
	constants := productTypeConstants(t)

	known := make(map[string]bool, len(constants))
	for name, productType := range constants {
		known[productType] = true
		t.Run(name, func(t *testing.T) {
			permission, ok := ProductTypeReadPermission[productType]
			assert.True(t, ok, "%s is missing from ProductTypeReadPermission", name)
			assert.NotEmpty(t, permission, "%s has no read permission", name)
		})
	}

	for productType := range ProductTypeReadPermission {
		assert.True(t, known[productType], "ProductTypeReadPermission has unknown product type %q", productType)
	}
}
