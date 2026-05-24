//revive:disable:dot-imports
package v1_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectorService dispatch structure", func() {
	var (
		fileSet                       *token.FileSet
		connectorServiceFile          *ast.File
		requestConnectorKindSwitches  []string
		connectorKindVariableSwitches []string
	)

	BeforeEach(func() {
		sourceBytes, err := os.ReadFile("connector_service.go")
		Expect(err).ToNot(HaveOccurred())
		fileSet = token.NewFileSet()
		connectorServiceFile, err = parser.ParseFile(fileSet, "connector_service.go", sourceBytes, 0)
		Expect(err).ToNot(HaveOccurred())

		requestConnectorKindSwitches = nil
		connectorKindVariableSwitches = nil
		for _, declaration := range connectorServiceFile.Decls {
			funcDecl, ok := declaration.(*ast.FuncDecl)
			if !ok || funcDecl.Body == nil {
				continue
			}
			requestKindVariables := map[string]struct{}{}
			ast.Inspect(funcDecl.Body, func(node ast.Node) bool {
				switch statement := node.(type) {
				case *ast.AssignStmt:
					for i, expr := range statement.Rhs {
						if i >= len(statement.Lhs) || !isGetConnectorKindCall(expr) {
							continue
						}
						if ident, ok := statement.Lhs[i].(*ast.Ident); ok {
							requestKindVariables[ident.Name] = struct{}{}
						}
					}
				case *ast.ValueSpec:
					for i, expr := range statement.Values {
						if i >= len(statement.Names) || !isGetConnectorKindCall(expr) {
							continue
						}
						requestKindVariables[statement.Names[i].Name] = struct{}{}
					}
				default:
				}
				switchStmt, ok := node.(*ast.SwitchStmt)
				if !ok {
					return true
				}
				position := fileSet.Position(switchStmt.Pos()).String()
				if call, ok := switchStmt.Tag.(*ast.CallExpr); ok {
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "GetConnectorKind" {
						requestConnectorKindSwitches = append(requestConnectorKindSwitches, position)
					}
				}
				if selector, ok := switchStmt.Tag.(*ast.Ident); ok && selector.Name == "kind" {
					connectorKindVariableSwitches = append(connectorKindVariableSwitches, funcDecl.Name.Name)
				}
				if selector, ok := switchStmt.Tag.(*ast.Ident); ok {
					if _, found := requestKindVariables[selector.Name]; found {
						requestConnectorKindSwitches = append(requestConnectorKindSwitches, position)
					}
				}
				return true
			})
		}
	})

	Describe("connector_kind routing", func() {
		It("should not dispatch handlers by switching on request connector_kind", func() {
			Expect(requestConnectorKindSwitches).To(BeEmpty())
		})

		It("should centralize connector_kind switching in connectorFor", func() {
			Expect(connectorKindVariableSwitches).To(ConsistOf("connectorFor"))
		})
	})
})

func isGetConnectorKindCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "GetConnectorKind"
}
