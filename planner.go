package reviewplanner

import "sort"

// Plan 根据输入求解派评方案。
//
// 存在合规完整方案时返回 Feasible=true 的最优方案；
// 否则返回 Feasible=false，绝不返回部分派评。
func Plan(in Input) Result {
	p, ok := newPlanner(in)
	if !ok {
		return Result{Feasible: false}
	}
	return p.solve()
}

// planner 保存一次求解的中间状态。
type planner struct {
	in        Input
	matIndex  map[string]int
	revIndex  map[string]int
	avoidSets []map[string]bool
	prevPairs map[[2]string]bool

	lockedByMat [][]string // 每份材料被锁定的评审员
	lockedDepts []map[string]bool
	lockedByRev []int // 每名评审员被锁定的次数
}

func newPlanner(in Input) (*planner, bool) {
	p := &planner{
		in:          in,
		matIndex:    make(map[string]int, len(in.Materials)),
		revIndex:    make(map[string]int, len(in.Reviewers)),
		avoidSets:   make([]map[string]bool, len(in.Materials)),
		prevPairs:   make(map[[2]string]bool),
		lockedByMat: make([][]string, len(in.Materials)),
		lockedDepts: make([]map[string]bool, len(in.Materials)),
		lockedByRev: make([]int, len(in.Reviewers)),
	}
	for i, m := range in.Materials {
		if _, dup := p.matIndex[m.ID]; dup {
			return nil, false
		}
		p.matIndex[m.ID] = i
		set := make(map[string]bool, len(m.Avoid))
		for _, id := range m.Avoid {
			set[id] = true
		}
		p.avoidSets[i] = set
		p.lockedDepts[i] = make(map[string]bool)
	}
	for i, r := range in.Reviewers {
		if _, dup := p.revIndex[r.ID]; dup {
			return nil, false
		}
		p.revIndex[r.ID] = i
	}
	for _, a := range in.Previous {
		if _, ok := p.matIndex[a.MaterialID]; !ok {
			continue
		}
		if _, ok := p.revIndex[a.ReviewerID]; !ok {
			continue
		}
		p.prevPairs[[2]string{a.MaterialID, a.ReviewerID}] = true
	}
	if !p.applyLocks(in.Locks) {
		return nil, false
	}
	return p, true
}

// applyLocks 校验并预分配人工锁定；任何锁定与回避、部门或容量冲突即无解。
func (p *planner) applyLocks(locks []Lock) bool {
	seen := make(map[[2]string]bool, len(locks))
	for _, l := range locks {
		mi, ok := p.matIndex[l.MaterialID]
		if !ok {
			return false
		}
		ri, ok := p.revIndex[l.ReviewerID]
		if !ok {
			return false
		}
		key := [2]string{l.MaterialID, l.ReviewerID}
		if seen[key] {
			continue // 完全相同的锁定视为一条
		}
		seen[key] = true
		if p.avoidSets[mi][l.ReviewerID] {
			return false
		}
		dept := p.in.Reviewers[ri].Department
		if p.lockedDepts[mi][dept] {
			return false // 同一份材料的两条锁定占了同一部门席位
		}
		if len(p.lockedByMat[mi]) >= seatsPerMaterial {
			return false
		}
		p.lockedDepts[mi][dept] = true
		p.lockedByMat[mi] = append(p.lockedByMat[mi], l.ReviewerID)
		p.lockedByRev[ri]++
		if p.in.Reviewers[ri].BaseLoad+p.lockedByRev[ri] > p.in.Reviewers[ri].MaxLoad {
			return false
		}
	}
	return true
}

const seatsPerMaterial = 3

// solve 建图求解并汇总结果。
func (p *planner) solve() Result {
	mats, revs := p.in.Materials, p.in.Reviewers
	nMat, nRev := len(mats), len(revs)

	// 节点布局：source | 材料 | (材料,部门) | 评审员 | sink
	source := 0
	matBase := 1
	deptBase := matBase + nMat
	type deptSlot struct {
		mat  int
		dept string
	}
	var slots []deptSlot
	for mi := range mats {
		deptSet := make(map[string]bool)
		for _, r := range revs {
			deptSet[r.Department] = true
		}
		depts := make([]string, 0, len(deptSet))
		for d := range deptSet {
			depts = append(depts, d)
		}
		sort.Strings(depts)
		for _, d := range depts {
			if p.lockedDepts[mi][d] {
				continue // 该部门席位已被锁定占用
			}
			slots = append(slots, deptSlot{mat: mi, dept: d})
		}
	}
	revBase := deptBase + len(slots)
	sink := revBase + nRev
	flow := newMinCostFlow(sink + 1)

	// 第一目标权重：必须大于负载费用总和的可能波动范围。
	var bigWeight int64 = 1
	for _, r := range revs {
		bigWeight += int64(r.MaxLoad) * int64(r.MaxLoad)
	}

	totalLocks := 0
	for mi := range mats {
		remain := seatsPerMaterial - len(p.lockedByMat[mi])
		totalLocks += len(p.lockedByMat[mi])
		flow.addEdge(source, matBase+mi, remain, 0, -1)
	}
	for si, s := range slots {
		flow.addEdge(matBase+s.mat, deptBase+si, 1, 0, -1)
		for ri, r := range revs {
			if r.Department != s.dept || p.avoidSets[s.mat][r.ID] {
				continue
			}
			if contains(p.lockedByMat[s.mat], r.ID) {
				continue // 已预分配，避免重复配对
			}
			var cost int64
			if p.prevPairs[[2]string{mats[s.mat].ID, r.ID}] {
				cost = -bigWeight
			}
			flow.addEdge(deptBase+si, revBase+ri, 1, cost, si*nRev+ri)
		}
	}
	for ri, r := range revs {
		remaining := r.MaxLoad - r.BaseLoad - p.lockedByRev[ri]
		for k := p.lockedByRev[ri] + 1; k <= remaining+p.lockedByRev[ri]; k++ {
			// 第 k 个本批次名额的边际平方费用：(BaseLoad+k)^2-(BaseLoad+k-1)^2
			load := int64(r.BaseLoad + k)
			flow.addEdge(revBase+ri, sink, 1, 2*load-1, -1)
		}
	}

	need := seatsPerMaterial*nMat - totalLocks
	got, _ := flow.augment(source, sink, need)
	if got < need {
		return Result{Feasible: false}
	}

	assignments := make([]Assignment, 0, seatsPerMaterial*nMat)
	for mi := range mats {
		for _, rid := range p.lockedByMat[mi] {
			assignments = append(assignments, Assignment{MaterialID: mats[mi].ID, ReviewerID: rid})
		}
	}
	for si, s := range slots {
		for _, e := range flow.graph[deptBase+si] {
			if e.pairIdx >= 0 && e.cap == 0 {
				ri := e.pairIdx - si*nRev
				assignments = append(assignments, Assignment{MaterialID: mats[s.mat].ID, ReviewerID: revs[ri].ID})
			}
		}
	}
	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].MaterialID != assignments[j].MaterialID {
			return assignments[i].MaterialID < assignments[j].MaterialID
		}
		return assignments[i].ReviewerID < assignments[j].ReviewerID
	})

	res := Result{Feasible: true, Assignments: assignments}
	loads := make([]int, nRev)
	for i, r := range revs {
		loads[i] = r.BaseLoad
	}
	for _, a := range assignments {
		loads[p.revIndex[a.ReviewerID]]++
		if p.prevPairs[[2]string{a.MaterialID, a.ReviewerID}] {
			res.PreservedPairs++
		}
	}
	for _, l := range loads {
		res.LoadSquareSum += int64(l) * int64(l)
	}
	return res
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
