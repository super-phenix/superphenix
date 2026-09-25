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

// productTypeConstants parses the ProductType* declarations out of model.go, keyed by the resource Name.
func productTypeConstants(t *testing.T) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "model.go", nil, 0)
	require.NoError(t, err)

	constants := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "ProductType") {
					continue
				}
				composite, ok := valueSpec.Values[i].(*ast.CompositeLit)
				if !ok || !isResourceLiteral(composite) {
					continue // ProductTypeReadPermission and the like
				}
				constants[name.Name] = resourceName(t, name.Name, composite)
			}
		}
	}

	require.NotEmpty(t, constants, "no ProductType* declaration found in model.go")
	return constants
}

// isResourceLiteral tells whether the literal is a router.Resource.
func isResourceLiteral(composite *ast.CompositeLit) bool {
	selector, ok := composite.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "router" && selector.Sel.Name == "Resource"
}

// resourceName returns the Name field of a router.Resource literal.
func resourceName(t *testing.T, declaration string, composite *ast.CompositeLit) string {
	t.Helper()
	for _, element := range composite.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := pair.Key.(*ast.Ident); !ok || key.Name != "Name" {
			continue
		}
		lit, ok := pair.Value.(*ast.BasicLit)
		require.True(t, ok, "%s: Name must be a string literal", declaration)
		value, err := strconv.Unquote(lit.Value)
		require.NoError(t, err)
		return value
	}
	require.Fail(t, "resource has no Name", declaration)
	return ""
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
