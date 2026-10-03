package reviewplanner

import "testing"

// ---- 测试辅助 ----

// checkValid 校验方案完整合规：每材料 3 名不同评审员、不同部门、
// 不触回避、包含全部锁定、容量不超限。
func checkValid(t *testing.T, in Input, res Result) {
	t.Helper()
	dept := map[string]string{}
	cap := map[string]int{}
	exist := map[string]int{}
	for _, r := range in.Reviewers {
		dept[r.ID] = r.Department
		cap[r.ID] = r.Capacity
		exist[r.ID] = r.Existing
	}
	avoid := map[Assignment]bool{}
	for _, a := range in.Avoid {
		avoid[a] = true
	}
	byMat := map[string][]string{}
	for _, as := range res.Assignments {
		if avoid[as] {
			t.Errorf("assignment %v violates avoid list", as)
		}
		byMat[as.MaterialID] = append(byMat[as.MaterialID], as.ReviewerID)
	}
	for _, m := range in.Materials {
		rs := byMat[m.ID]
		if len(rs) != 3 {
			t.Fatalf("material %s has %d reviewers, want 3", m.ID, len(rs))
		}
		seen := map[string]bool{}
		seenDept := map[string]bool{}
		for _, r := range rs {
			if seen[r] {
				t.Errorf("material %s: duplicate reviewer %s", m.ID, r)
			}
			seen[r] = true
			if seenDept[dept[r]] {
				t.Errorf("material %s: duplicate department %s", m.ID, dept[r])
			}
			seenDept[dept[r]] = true
		}
	}
	for _, lk := range in.Locks {
		found := false
		for _, r := range byMat[lk.MaterialID] {
			if r == lk.ReviewerID {
				found = true
			}
		}
		if !found {
			t.Errorf("lock %s->%s missing from result", lk.MaterialID, lk.ReviewerID)
		}
	}
	load := map[string]int{}
	for _, as := range res.Assignments {
		load[as.ReviewerID]++
	}
	for id, l := range load {
		if exist[id]+l > cap[id] {
			t.Errorf("reviewer %s overloaded: %d+%d > %d", id, exist[id], l, cap[id])
		}
	}
}

// keptPairs 统计方案保留的旧配对数。
func keptPairs(in Input, res Result) int {
	prev := map[Assignment]bool{}
	for _, a := range in.Previous {
		prev[a] = true
	}
	n := 0
	for _, as := range res.Assignments {
		if prev[as] {
			n++
		}
	}
	return n
}

// bruteForce 独立暴力枚举全部合规完整方案，返回最优目标值。
// 用于在小批次上核对 Plan 的两个目标值。
func bruteForce(in Input) (bestKept, bestLoad2 int, feasible bool) {
	dept := map[string]string{}
	rids := []string{}
	for _, r := range in.Reviewers {
		dept[r.ID] = r.Department
		rids = append(rids, r.ID)
	}
	avoid := map[Assignment]bool{}
	for _, a := range in.Avoid {
		avoid[a] = true
	}
	prev := map[Assignment]bool{}
	for _, a := range in.Previous {
		prev[a] = true
	}
	// 每份材料的全部可行三元组（不考虑容量）。
	type triple [3]string
	combos := map[string][]triple{}
	for _, m := range in.Materials {
		var list []triple
		for i := 0; i < len(rids); i++ {
			for j := i + 1; j < len(rids); j++ {
				for k := j + 1; k < len(rids); k++ {
					t3 := triple{rids[i], rids[j], rids[k]}
					if dept[t3[0]] == dept[t3[1]] || dept[t3[0]] == dept[t3[2]] || dept[t3[1]] == dept[t3[2]] {
						continue
					}
					bad := false
					for _, r := range t3 {
						if avoid[Assignment{m.ID, r}] {
							bad = true
						}
					}
					for _, lk := range in.Locks {
						if lk.MaterialID == m.ID && lk.ReviewerID != t3[0] && lk.ReviewerID != t3[1] && lk.ReviewerID != t3[2] {
							bad = true
						}
					}
					if !bad {
						list = append(list, t3)
					}
				}
			}
		}
		combos[m.ID] = list
	}
	load := map[string]int{}
	bestKept, bestLoad2 = -1, 0
	var dfs func(mi int, kept int)
	dfs = func(mi int, kept int) {
		if mi == len(in.Materials) {
			load2 := 0
			for _, r := range in.Reviewers {
				tot := r.Existing + load[r.ID]
				load2 += tot * tot
			}
			if kept > bestKept || (kept == bestKept && (!feasible || load2 < bestLoad2)) {
				bestKept, bestLoad2, feasible = kept, load2, true
			}
			return
		}
		m := in.Materials[mi]
		for _, t3 := range combos[m.ID] {
			ok := true
			for _, r := range t3 {
				for _, rv := range in.Reviewers {
					if rv.ID == r && rv.Existing+load[r]+1 > rv.Capacity {
						ok = false
					}
				}
			}
			if !ok {
				continue
			}
			k := kept
			for _, r := range t3 {
				load[r]++
				if prev[Assignment{m.ID, r}] {
					k++
				}
			}
			dfs(mi+1, k)
			for _, r := range t3 {
				load[r]--
			}
		}
	}
	dfs(0, 0)
	return bestKept, bestLoad2, feasible
}
