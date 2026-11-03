package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/pkg/tree"
)

type testNode struct {
	ID       uint64
	ParentID uint64
	Name     string
	Children []*testNode
}

func build(items []*testNode) []*testNode {
	return tree.Build(items, tree.Options[*testNode]{
		ID:       func(n *testNode) uint64 { return n.ID },
		ParentID: func(n *testNode) uint64 { return n.ParentID },
		SetChildren: func(n *testNode, kids []*testNode) {
			n.Children = kids
		},
	})
}

func TestBuild(t *testing.T) {
	roots := build([]*testNode{
		{ID: 1, Name: "root"},
		{ID: 2, ParentID: 1, Name: "child-a"},
		{ID: 3, ParentID: 1, Name: "child-b"},
		{ID: 4, ParentID: 2, Name: "grandchild"},
	})
	assert.Len(t, roots, 1)
	assert.Equal(t, "root", roots[0].Name)
	assert.Len(t, roots[0].Children, 2)
	assert.Equal(t, "child-a", roots[0].Children[0].Name)
	assert.Equal(t, "grandchild", roots[0].Children[0].Children[0].Name)
}

func TestBuildOrphansSelfAndCycles(t *testing.T) {
	roots := build([]*testNode{
		{ID: 1, Name: "orphan", ParentID: 99},
		{ID: 2, Name: "self", ParentID: 2},
		{ID: 3, Name: "cycle-a", ParentID: 4},
		{ID: 4, Name: "cycle-b", ParentID: 3},
	})
	assert.Len(t, roots, 4)

	empty := build(nil)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestBuildReachesCyclePath(t *testing.T) {
	// 5 -> 6 -> 7 -> 5 is a cycle; 8 hangs below 5 and must stay visible.
	roots := build([]*testNode{
		{ID: 5, ParentID: 7},
		{ID: 6, ParentID: 5},
		{ID: 7, ParentID: 6},
		{ID: 8, ParentID: 5},
	})
	assert.Len(t, roots, 3)
	var found bool
	for _, root := range roots {
		if root.ID == 5 {
			found = true
			var hasEight bool
			for _, child := range root.Children {
				if child.ID == 8 {
					hasEight = true
				}
			}
			assert.True(t, hasEight)
		}
	}
	assert.True(t, found)
}
