package reviewplanner

import (
	"errors"
	"fmt"
	"sort"
)

// ErrNoSolution 表示不存在合规完整方案。
var ErrNoSolution = errors.New("reviewplanner: no feasible complete assignment")

// Plan 根据输入规划完整派评方案。
// 存在合规完整方案时返回方案与目标值；否则返回 ErrNoSolution。
// 第一目标：保留最多旧方案配对；第二目标：分配后总在办量平方和最小。
func Plan(in Input) (Result, Stats, error) {
	p, err := newProblem(in)
	if err != nil {
		return Result{}, Stats{}, err
	}
	return p.solve()
}

// pair 标识一条材料-评审员配对。
type pair struct {
	material int
	reviewer int
}

// combo 一份材料的一种可行席位组合（3 名评审员的下标，升序）。
type combo struct {
	reviewers [3]int
	kept      int // 该组合保留的旧方案配对数
}

type problem struct {
	nMaterials int
	nReviewers int
	existing   []int
	capacity   []int
	materialID []string
	reviewerID []string
	// 每份材料的候选组合，按材料下标（已按候选数升序重排）。
	combos [][]combo
	// suffixKeptUB[i]：第 i..n-1 份材料最多还能保留的旧配对数上界。
	suffixKeptUB []int
	// perm 搜索顺序 -> 原材料下标。
	perm []int

	bestKept  int
	bestLoad2 int
	best      []int // 每份材料选中的组合下标
	found     bool
}

func newProblem(in Input) (*problem, error) {
	if len(in.Materials) == 0 {
		return nil, errors.New("reviewplanner: material list is empty")
	}
	reviewerIdx := make(map[string]int, len(in.Reviewers))
	for i, r := range in.Reviewers {
		if _, dup := reviewerIdx[r.ID]; dup {
			return nil, fmt.Errorf("reviewplanner: duplicate reviewer id %q", r.ID)
		}
		reviewerIdx[r.ID] = i
	}
	materialIdx := make(map[string]int, len(in.Materials))
	for i, m := range in.Materials {
		if _, dup := materialIdx[m.ID]; dup {
			return nil, fmt.Errorf("reviewplanner: duplicate material id %q", m.ID)
		}
		materialIdx[m.ID] = i
	}

	n, m := len(in.Materials), len(in.Reviewers)
	p := &problem{
		nMaterials: n,
		nReviewers: m,
		materialID: make([]string, n),
		reviewerID: make([]string, m),
	}
	for i, mt := range in.Materials {
		p.materialID[i] = mt.ID
	}
	for i, r := range in.Reviewers {
		p.reviewerID[i] = r.ID
		p.existing = append(p.existing, r.Existing)
		p.capacity = append(p.capacity, r.Capacity)
		if r.Existing > r.Capacity {
			return nil, fmt.Errorf("reviewplanner: reviewer %q existing load exceeds capacity", r.ID)
		}
	}

	// 回避名单。
	avoid := make(map[pair]bool)
	for _, a := range in.Avoid {
		mi, ok := materialIdx[a.MaterialID]
		if !ok {
			return nil, fmt.Errorf("reviewplanner: avoid list references unknown material %q", a.MaterialID)
		}
		ri, ok := reviewerIdx[a.ReviewerID]
		if !ok {
			return nil, fmt.Errorf("reviewplanner: avoid list references unknown reviewer %q", a.ReviewerID)
		}
		avoid[pair{mi, ri}] = true
	}

	// 人工锁定。锁定不能绕过回避、部门或容量限制：
	// 与回避冲突或彼此矛盾的锁定使问题无解。
	locks := make(map[int][]int) // material -> locked reviewers
	for _, lk := range in.Locks {
		mi, ok := materialIdx[lk.MaterialID]
		if !ok {
			return nil, fmt.Errorf("reviewplanner: lock references unknown material %q", lk.MaterialID)
		}
		ri, ok := reviewerIdx[lk.ReviewerID]
		if !ok {
			return nil, fmt.Errorf("reviewplanner: lock references unknown reviewer %q", lk.ReviewerID)
		}
		if avoid[pair{mi, ri}] {
			return nil, ErrNoSolution
		}
		dup := false
		for _, prev := range locks[mi] {
			if prev == ri {
				dup = true
				break
			}
		}
		if !dup {
			locks[mi] = append(locks[mi], ri)
		}
	}

	// 旧方案配对集合（仅统计仍有效的材料与评审员）。
	prevPairs := make(map[pair]bool)
	for _, a := range in.Previous {
		mi, ok1 := materialIdx[a.MaterialID]
		ri, ok2 := reviewerIdx[a.ReviewerID]
		if ok1 && ok2 {
			prevPairs[pair{mi, ri}] = true
		}
	}

	// 为每份材料枚举候选组合。
	dept := make([]string, m)
	for i, r := range in.Reviewers {
		dept[i] = r.Department
	}
	type matCombos struct {
		idx    int
		combos []combo
	}
	ordered := make([]matCombos, 0, n)
	for mi := 0; mi < n; mi++ {
		cs, err := enumerateCombos(mi, m, dept, avoid, locks[mi], prevPairs)
		if err != nil {
			return nil, fmt.Errorf("reviewplanner: material %q: %w", p.materialID[mi], err)
		}
		if len(cs) == 0 {
			return nil, ErrNoSolution
		}
		ordered = append(ordered, matCombos{mi, cs})
	}
	// 候选少的材料先排，加速收敛。
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(ordered[i].combos) < len(ordered[j].combos)
	})
	p.combos = make([][]combo, n)
	perm := make([]int, n) // 搜索顺序 -> 原材料下标
	for i, mc := range ordered {
		p.combos[i] = mc.combos
		perm[i] = mc.idx
	}
	p.perm = perm

	p.suffixKeptUB = make([]int, n+1)
	for i := n - 1; i >= 0; i-- {
		maxKept := 0
		for _, c := range p.combos[i] {
			if c.kept > maxKept {
				maxKept = c.kept
			}
		}
		p.suffixKeptUB[i] = p.suffixKeptUB[i+1] + maxKept
	}
	return p, nil
}

// enumerateCombos 枚举一份材料的全部可行 3 人组合：
// 3 人来自不同部门、不在回避名单、包含全部锁定评审员。
func enumerateCombos(mi, nReviewers int, dept []string, avoid map[pair]bool, locked []int, prevPairs map[pair]bool) ([]combo, error) {
	if len(locked) > 3 {
		return nil, ErrNoSolution
	}
	for i := 0; i < len(locked); i++ {
		for j := i + 1; j < len(locked); j++ {
			if dept[locked[i]] == dept[locked[j]] {
				return nil, ErrNoSolution
			}
		}
	}
	var out []combo
	for a := 0; a < nReviewers; a++ {
		for b := a + 1; b < nReviewers; b++ {
			if dept[a] == dept[b] {
				continue
			}
			for c := b + 1; c < nReviewers; c++ {
				if dept[a] == dept[c] || dept[b] == dept[c] {
					continue
				}
				if avoid[pair{mi, a}] || avoid[pair{mi, b}] || avoid[pair{mi, c}] {
					continue
				}
				ok := true
				for _, r := range locked {
					if r != a && r != b && r != c {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				kept := 0
				for _, r := range [3]int{a, b, c} {
					if prevPairs[pair{mi, r}] {
						kept++
					}
				}
				out = append(out, combo{reviewers: [3]int{a, b, c}, kept: kept})
			}
		}
	}
	return out, nil
}
