// Package tree 把扁平的父子列表组装成树。
package tree

// Options 描述如何读取节点标识、父标识并写入子节点，
// 使调用方可以保留自己的 JSON 结构（内嵌 model + children）。
type Options[T any] struct {
	ID          func(T) uint64
	ParentID    func(T) uint64
	SetChildren func(T, []T)
}

// Build 把扁平列表构建为树，并保持输入顺序。
// 孤儿节点、自引用和环都会被提升为根节点，保证任何一条数据都不会丢失。
func Build[T any](items []T, opts Options[T]) []T {
	n := len(items)
	if n == 0 {
		return []T{}
	}
	index := make(map[uint64]int, n)
	for i, item := range items {
		index[opts.ID(item)] = i
	}
	// parent[i] 为父节点下标，-1 表示根节点。
	parent := make([]int, n)
	for i, item := range items {
		pid := opts.ParentID(item)
		if idx, ok := index[pid]; ok && idx != i {
			parent[i] = idx
		} else {
			parent[i] = -1
		}
	}

	// 标记处于环中的节点，并把它们提升为根，避免递归死循环。
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

// inCycle 判断从 start 沿父链向上是否能回到自己。
// 最多遍历 len(parent) 步，保证脏数据下也能终止。
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
