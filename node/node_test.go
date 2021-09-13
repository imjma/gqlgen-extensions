package node_test

import (
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/imjma/gqlgen-extensions/node"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

var schema = gqlparser.MustLoadSchema(
	&ast.Source{
		Name: "test.graphql",
		Input: `
		interface NameInterface {
			name: String
		}
		type Item implements NameInterface {
			scalar: String
			name: String
			list(size: Int = 10): [Item]
		}
		type ExpensiveItem implements NameInterface {
			name: String
		}
		type Named {
			name: String
		}
		union NameUnion = Item | Named
		type Query {
			scalar: String
			object: Item
			interface: NameInterface
			union: NameUnion
			customObject: Item
			list(size: Int = 10): [Item]
		}
		`,
	},
)

func calculate(source string) int {
	query := gqlparser.MustLoadQuery(schema, source)

	es := &graphql.ExecutableSchemaMock{
		ComplexityFunc: func(typeName, field string, childComplexity int, args map[string]interface{}) (int, bool) {
			switch typeName + "." + field {
			case "ExpensiveItem.name":
				return 5, true
			case "Query.list", "Item.list":
				return int(args["size"].(int64)) + childComplexity, true
			case "Query.customObject":
				return 1, true
			}
			return 0, false
		},
		SchemaFunc: func() *ast.Schema {
			return schema
		},
	}

	return node.Calculate(es, query.Operations[0], nil)
}

func TestCalculate(t *testing.T) {
	// return 0 for non size field
	query := `{
			scalar
		}`
	node := calculate(query)
	assert.Equal(t, 0, node)

	query = `
		{
			scalar1: scalar
			scalar2: scalar
		}
		`
	node = calculate(query)
	assert.Equal(t, 0, node)

	// use default size for node limit
	query = `
		{
			list {
				scalar
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 10, node)

	// use given size for node limit
	query = `
		{
			list(size: 100) {
				scalar
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 100, node)

	// return size*size
	query = `
		{
			list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 150, node)

	// return size*size + size*size
	query = `
		{
			a:list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
			b:list(size: 6) {
				list(size: 3) {
					scalar
				}
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 168, node)

	// return size*size + 0
	query = `
		{
			a:list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
			b:scalar
		}
		`
	node = calculate(query)
	assert.Equal(t, 150, node)

	// return size*size + 0 + size
	query = `
		{
			a:list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
			b:scalar
			c:list(size:5) {
				scalar
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 155, node)

	// ignore negative size
	query = `
		{
			list(size: -100) {
				scalar
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 0, node)
}

func TestFragment(t *testing.T) {
	// return 0 for fragment
	query := `
		{
			... Fragment
		}
		fragment Fragment on Query {
			scalar
		}
		`
	node := calculate(query)
	assert.Equal(t, 0, node)

	// return size*size for fragment
	query = `
		{
			... Fragment
		}
		fragment Fragment on Query {
			list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 150, node)

	// return size*size + size*size for fragment
	query = `
		{
			... Fragment
		}
		fragment Fragment on Query {
			a:list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
			b:list(size: 6) {
				list(size: 3) {
					scalar
				}
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 168, node)

	// return size*size for inline + size*size for fragment
	query = `
		{
			a:list(size: 2) {
				list(size: 5) {
					list(size: 15) {
						scalar
					}
				}
			}
			... Fragment
		}
		fragment Fragment on Query {
			b:list(size: 6) {
				list(size: 3) {
					scalar
				}
			}
		}
		`
	node = calculate(query)
	assert.Equal(t, 168, node)
}
