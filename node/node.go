package node

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

func Calculate(es graphql.ExecutableSchema, op *ast.OperationDefinition, vars map[string]interface{}) int {
	walker := nodeWalker{
		es:     es,
		schema: es.Schema(),
		vars:   vars,
	}
	return walker.selectionSetNode(op.SelectionSet)
}

type nodeWalker struct {
	es     graphql.ExecutableSchema
	schema *ast.Schema
	vars   map[string]interface{}
}

// selectionSetNode walk through the query and calcuate node
func (nw nodeWalker) selectionSetNode(selectionSet ast.SelectionSet) int {
	var node int
	for _, selection := range selectionSet {
		switch s := selection.(type) {
		case *ast.Field:
			fieldDefinition := nw.schema.Types[s.Definition.Type.Name()]
			var childNode int
			switch fieldDefinition.Kind {
			case ast.Object, ast.Interface, ast.Union:
				childNode = nw.selectionSetNode(s.SelectionSet)
			}

			args := s.ArgumentMap(nw.vars)
			var fieldNode int
			if s.ObjectDefinition.Kind == ast.Interface {
				fieldNode = nw.interfaceFieldNode(s.ObjectDefinition, s.Name, childNode, args)
			} else {
				fieldNode = nw.fieldNode(s.ObjectDefinition.Name, s.Name, childNode, args)
			}
			node = safeAdd(node, fieldNode)

		case *ast.FragmentSpread:
			node = safeAdd(node, nw.selectionSetNode(s.Definition.SelectionSet))

		case *ast.InlineFragment:
			node = safeAdd(node, nw.selectionSetNode(s.SelectionSet))
		}
	}
	return node
}

func (nw nodeWalker) interfaceFieldNode(def *ast.Definition, field string, childNode int, args map[string]interface{}) int {
	// Interfaces don't have their own separate field costs, so they have to assume the worst case.
	// We iterate over all implementors and choose the most expensive one.
	maxNode := 0
	implementors := nw.schema.GetPossibleTypes(def)
	for _, t := range implementors {
		fieldNode := nw.fieldNode(t.Name, field, childNode, args)
		if fieldNode > maxNode {
			maxNode = fieldNode
		}
	}
	return maxNode
}

func (nw nodeWalker) fieldNode(object, field string, childNode int, args map[string]interface{}) int {
	if customComplexity, ok := nw.es.Complexity(object, field, childNode, args); ok {
		// customComplexity need minus childNode to dedup number of node
		return safeMultiply(customComplexity-childNode, childNode)
	}
	// default node calculation
	return childNode
}

const maxInt = int(^uint(0) >> 1)

// safeAdd is a saturating add of a and b that ignores negative operands.
// If a + b would overflow through normal Go addition,
// it returns the maximum integer value instead.
//
// Adding complexities with this function prevents attackers from intentionally
// overflowing the complexity calculation to allow overly-complex queries.
//
// It also helps mitigate the impact of custom complexities that accidentally
// return negative values.
func safeAdd(a, b int) int {
	// Ignore negative operands.
	if a < 0 {
		if b < 0 {
			return 1
		}
		return b
	} else if b < 0 {
		return a
	}

	c := a + b
	if c < a {
		// Set c to maximum integer instead of overflowing.
		c = maxInt
	}
	return c
}

// safeMultiply is a saturating multiply of a and b that ignores negative operands.
//
// Adding complexities with this function prevents attackers from intentionally
// overflowing the complexity calculation to allow overly-complex queries.
//
// It also helps mitigate the impact of custom complexities that accidentally
// return negative values.
func safeMultiply(a, b int) int {
	if a == 0 {
		a = 1
	}
	// Ignore negative operands.
	if a < 0 {
		if b <= 0 {
			return 0
		}
		return b
	} else if b <= 0 {
		return a
	}

	return a * b
}
