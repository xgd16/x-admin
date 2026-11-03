package tree

// Options describes how to read identifiers and write children on a concrete
// node type, so callers keep their own JSON shape (embedded model + children).
type Options[T any] struct {
	ID          func(T) uint64
	ParentID    func(T) uint64
	SetChildren func(T, []T)
}

// Build turns a flat list into a tree while preserving input order. Orphans,
// self references and cycles are promoted to roots so no item is ever lost.
func Build[T any](items []T, opts Options[T]) []T {
	n := len(items)
	if n == 0 {
		return []T{}
	}
	index := make(map[uint64]int, n)
	for i, item := range items {
		index[opts.ID(item)] = i
	}
	parent := make([]int, n)
	for i, item := range items {
		pid := opts.ParentID(item)
		if idx, ok := index[pid]; ok && idx != i {
			parent[i] = idx
		} else {
			parent[i] = -1
		}
	}

	cycle := make([]bool, n)
	for i := 0; i < n; i++ {
		cycle[i] = inCycle(i, parent)
	}
	for i := range parent {
		if cycle[i] {
			parent[i] = -1
		}
	}

	children := make(map[int][]int, n)
	roots := make([]int, 0)
	for i := range items {
		if parent[i] == -1 {
			roots = append(roots, i)
			continue
		}
		children[parent[i]] = append(children[parent[i]], i)
	}

	var build func(int) T
	build = func(i int) T {
		kids := make([]T, 0, len(children[i]))
		for _, ci := range children[i] {
			kids = append(kids, build(ci))
		}
		node := items[i]
		if opts.SetChildren != nil {
			opts.SetChildren(node, kids)
		}
		return node
	}

	out := make([]T, 0, len(roots))
	for _, root := range roots {
		out = append(out, build(root))
	}
	return out
}

func inCycle(start int, parent []int) bool {
	cur := parent[start]
	for steps := 0; steps < len(parent) && cur >= 0; steps++ {
		if cur == start {
			return true
		}
		cur = parent[cur]
	}
	return false
}
