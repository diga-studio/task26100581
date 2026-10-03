package reviewplanner

// solve 以分支限界搜索最优完整方案。
// 第一目标 kept 最大绝对优先；同 kept 时取总在办量平方和最小者。
func (p *problem) solve() (Result, Stats, error) {
	load := make([]int, p.nReviewers) // 本批次已分配量
	chosen := make([]int, p.nMaterials)
	p.bestKept = -1
	p.dfs(0, 0, load, chosen)
	if !p.found {
		return Result{}, Stats{}, ErrNoSolution
	}

	res := Result{Assignments: make([]Assignment, 0, p.nMaterials*3)}
	for i := 0; i < p.nMaterials; i++ {
		mi := p.perm[i]
		for _, r := range p.combos[i][p.best[i]].reviewers {
			res.Assignments = append(res.Assignments, Assignment{
				MaterialID: p.materialID[mi],
				ReviewerID: p.reviewerID[r],
			})
		}
	}
	return res, Stats{KeptPairs: p.bestKept, LoadSquared: p.bestLoad2}, nil
}

// dfs 按搜索顺序为第 pos 份材料选择组合。
func (p *problem) dfs(pos, kept int, load []int, chosen []int) {
	if kept+p.suffixKeptUB[pos] < p.bestKept {
		return
	}
	if pos == p.nMaterials {
		load2 := 0
		for i := 0; i < p.nReviewers; i++ {
			total := p.existing[i] + load[i]
			load2 += total * total
		}
		if kept > p.bestKept || (kept == p.bestKept && (!p.found || load2 < p.bestLoad2)) {
			p.bestKept = kept
			p.bestLoad2 = load2
			if p.best == nil {
				p.best = make([]int, p.nMaterials)
			}
			copy(p.best, chosen)
			p.found = true
		}
		return
	}
	for ci, c := range p.combos[pos] {
		ok := true
		for _, r := range c.reviewers {
			if p.existing[r]+load[r]+1 > p.capacity[r] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for _, r := range c.reviewers {
			load[r]++
		}
		chosen[pos] = ci
		p.dfs(pos+1, kept+c.kept, load, chosen)
		for _, r := range c.reviewers {
			load[r]--
		}
	}
}
