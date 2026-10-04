package reviewplanner

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// bruteForceOptimum 用纯枚举独立计算两个目标的最优值，用于核对 Plan 的结果。
// 仅适用于极小批次。返回 (是否可行, 最大保留配对数, 最小负载平方和)。
func bruteForceOptimum(in Input) (bool, int, int64) {
	locksByMat := make(map[string][]string)
	for _, l := range in.Locks {
		locksByMat[l.MaterialID] = append(locksByMat[l.MaterialID], l.ReviewerID)
	}
	revByID := make(map[string]Reviewer)
	for _, r := range in.Reviewers {
		revByID[r.ID] = r
	}
	prev := make(map[[2]string]bool)
	for _, a := range in.Previous {
		prev[[2]string{a.MaterialID, a.ReviewerID}] = true
	}

	// 为每份材料枚举所有包含全部锁定配对的合法三人组合。
	combosPerMat := make([][][]string, len(in.Materials))
	for mi, m := range in.Materials {
		avoid := make(map[string]bool)
		for _, id := range m.Avoid {
			avoid[id] = true
		}
		locked := locksByMat[m.ID]
		for _, rid := range locked {
			if avoid[rid] {
				return false, 0, 0 // 锁定与回避冲突
			}
		}
		var pool []string
		for _, r := range in.Reviewers {
			if !avoid[r.ID] {
				pool = append(pool, r.ID)
			}
		}
		var combos [][]string
		var pick func(start int, chosen []string)
		pick = func(start int, chosen []string) {
			if len(chosen) == 3 {
				depts := make(map[string]bool)
				ok := true
				for _, id := range chosen {
					d := revByID[id].Department
					if depts[d] {
						ok = false
						break
					}
					depts[d] = true
				}
				if ok {
					combos = append(combos, append([]string(nil), chosen...))
				}
				return
			}
			for i := start; i < len(pool); i++ {
				pick(i+1, append(chosen, pool[i]))
			}
		}
		pick(0, nil)
		for _, c := range combos {
			ok := true
			for _, rid := range locked {
				found := false
				for _, id := range c {
					if id == rid {
						found = true
						break
					}
				}
				if !found {
					ok = false
					break
				}
			}
			if ok {
				combosPerMat[mi] = append(combosPerMat[mi], c)
			}
		}
		if len(combosPerMat[mi]) == 0 {
			return false, 0, 0
		}
	}

	bestPreserved := -1
	var bestSquare int64
	feasible := false
	loads := make(map[string]int)
	for _, r := range in.Reviewers {
		loads[r.ID] = r.BaseLoad
	}
	var search func(mi int, preserved int)
	search = func(mi int, preserved int) {
		if mi == len(in.Materials) {
			var square int64
			for _, r := range in.Reviewers {
				l := int64(loads[r.ID])
				square += l * l
			}
			if !feasible || preserved > bestPreserved ||
				(preserved == bestPreserved && square < bestSquare) {
				feasible = true
				bestPreserved = preserved
				bestSquare = square
			}
			return
		}
		for _, combo := range combosPerMat[mi] {
			ok := true
			for _, id := range combo {
				if loads[id]+1 > revByID[id].MaxLoad {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			add := 0
			for _, id := range combo {
				loads[id]++
				if prev[[2]string{in.Materials[mi].ID, id}] {
					add++
				}
			}
			search(mi+1, preserved+add)
			for _, id := range combo {
				loads[id]--
			}
		}
	}
	search(0, 0)
	return feasible, bestPreserved, bestSquare
}

// checkValid 校验方案完整合规，返回错误信息或空串。
func checkValid(in Input, res Result) string {
	if !res.Feasible {
		return "expected feasible result"
	}
	revByID := make(map[string]Reviewer)
	for _, r := range in.Reviewers {
		revByID[r.ID] = r
	}
	byMat := make(map[string][]string)
	for _, a := range res.Assignments {
		if _, ok := revByID[a.ReviewerID]; !ok {
			return "unknown reviewer " + a.ReviewerID
		}
		byMat[a.MaterialID] = append(byMat[a.MaterialID], a.ReviewerID)
	}
	matByID := make(map[string]Material)
	for _, m := range in.Materials {
		matByID[m.ID] = m
	}
	for _, m := range in.Materials {
		revs := byMat[m.ID]
		if len(revs) != 3 {
			return "material " + m.ID + " does not have exactly 3 reviewers"
		}
		seen := make(map[string]bool)
		depts := make(map[string]bool)
		avoid := make(map[string]bool)
		for _, id := range m.Avoid {
			avoid[id] = true
		}
		for _, rid := range revs {
			if seen[rid] {
				return "duplicate reviewer " + rid + " on " + m.ID
			}
			seen[rid] = true
			if avoid[rid] {
				return "avoided reviewer " + rid + " on " + m.ID
			}
			d := revByID[rid].Department
			if depts[d] {
				return "department " + d + " repeated on " + m.ID
			}
			depts[d] = true
		}
	}
	for _, l := range in.Locks {
		found := false
		for _, rid := range byMat[l.MaterialID] {
			if rid == l.ReviewerID {
				found = true
			}
		}
		if !found {
			return "lock not honored: " + l.MaterialID + "/" + l.ReviewerID
		}
	}
	loads := make(map[string]int)
	for _, a := range res.Assignments {
		loads[a.ReviewerID]++
	}
	for _, r := range in.Reviewers {
		allowance := r.MaxLoad - r.BaseLoad
		if allowance < 0 {
			allowance = 0 // 已有在办量本身超限，本批次名额为 0
		}
		if loads[r.ID] > allowance {
			return "reviewer " + r.ID + " over capacity"
		}
	}
	return ""
}

// planAndCheck 运行 Plan 并做完整性与最优性核对。
func planAndCheck(t *testing.T, in Input) Result {
	t.Helper()
	res := Plan(in)
	if msg := checkValid(in, res); msg != "" {
		t.Fatalf("invalid plan: %s", msg)
	}
	feasible, preserved, square := bruteForceOptimum(in)
	if !feasible {
		t.Fatalf("solver found a plan but brute force reports infeasible")
	}
	if res.PreservedPairs != preserved {
		t.Fatalf("preserved pairs = %d, want %d", res.PreservedPairs, preserved)
	}
	if res.LoadSquareSum != square {
		t.Fatalf("load square sum = %d, want %d", res.LoadSquareSum, square)
	}
	return res
}

func assignmentsOf(res Result) map[[2]string]bool {
	set := make(map[[2]string]bool)
	for _, a := range res.Assignments {
		set[[2]string{a.MaterialID, a.ReviewerID}] = true
	}
	return set
}

func sortedIDs(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// TestReplanLockForcesCascade 候选交错且含人工锁定：
// 锁定 M2-A 后，为填满 M2 的席位必须连带调整 M1 的安排。
func TestReplanLockForcesCascade(t *testing.T) {
	reviewers := []Reviewer{
		{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 1},
		{ID: "B", Department: "D2", BaseLoad: 0, MaxLoad: 1},
		{ID: "C", Department: "D3", BaseLoad: 0, MaxLoad: 1},
		{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 1},
		{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 1},
		{ID: "F", Department: "D3", BaseLoad: 0, MaxLoad: 1},
	}
	in := Input{
		Materials: []Material{
			{ID: "M1", Avoid: []string{"F"}},
			{ID: "M2"},
		},
		Reviewers: reviewers,
		Locks:     []Lock{{MaterialID: "M2", ReviewerID: "A"}},
		Previous: []Assignment{
			{MaterialID: "M1", ReviewerID: "A"},
			{MaterialID: "M1", ReviewerID: "B"},
			{MaterialID: "M1", ReviewerID: "C"},
			{MaterialID: "M2", ReviewerID: "D"},
			{MaterialID: "M2", ReviewerID: "E"},
			{MaterialID: "M2", ReviewerID: "F"},
		},
	}
	res := planAndCheck(t, in)
	// 每人容量 1，A 被锁到 M2 后 M1 必须换人；最优保留 4 对：
	// M1={B,C,D}，M2={A,E,F}。
	if res.PreservedPairs != 4 {
		t.Fatalf("preserved = %d, want 4", res.PreservedPairs)
	}
	got := assignmentsOf(res)
	for _, want := range [][2]string{
		{"M2", "A"}, {"M1", "B"}, {"M1", "C"}, {"M1", "D"}, {"M2", "E"}, {"M2", "F"},
	} {
		if !got[want] {
			t.Fatalf("missing pair %v in %v", want, res.Assignments)
		}
	}
}

// TestReplanNewAvoidCascade 重排时新增回避关系，需连带调整多份材料才能恢复合规。
func TestReplanNewAvoidCascade(t *testing.T) {
	reviewers := []Reviewer{
		{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 1},
		{ID: "B", Department: "D2", BaseLoad: 0, MaxLoad: 1},
		{ID: "C", Department: "D3", BaseLoad: 0, MaxLoad: 1},
		{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 1},
		{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 1},
		{ID: "F", Department: "D3", BaseLoad: 0, MaxLoad: 1},
	}
	previous := []Assignment{
		{MaterialID: "M1", ReviewerID: "A"},
		{MaterialID: "M1", ReviewerID: "B"},
		{MaterialID: "M1", ReviewerID: "C"},
		{MaterialID: "M2", ReviewerID: "D"},
		{MaterialID: "M2", ReviewerID: "E"},
		{MaterialID: "M2", ReviewerID: "F"},
	}
	in := Input{
		Materials: []Material{
			{ID: "M1", Avoid: []string{"A"}}, // 新增回避：A 不能再评 M1
			{ID: "M2"},
		},
		Reviewers: reviewers,
		Previous:  previous,
	}
	res := planAndCheck(t, in)
	// 每人容量 1：M1 只能由 D 顶替 A（D1 仅两人），M2 随之由 A 顶替 D。
	// 保留 4 对：M1={B,C,D}，M2={A,E,F}。
	if res.PreservedPairs != 4 {
		t.Fatalf("preserved = %d, want 4", res.PreservedPairs)
	}
	got := assignmentsOf(res)
	for _, want := range [][2]string{
		{"M1", "B"}, {"M1", "C"}, {"M1", "D"}, {"M2", "A"}, {"M2", "E"}, {"M2", "F"},
	} {
		if !got[want] {
			t.Fatalf("missing pair %v in %v", want, res.Assignments)
		}
	}
	if res.LoadSquareSum != 6 {
		t.Fatalf("load square sum = %d, want 6", res.LoadSquareSum)
	}
}

// TestPreservationBeatsBalance 第一目标绝对优先：
// 即使保留旧配对导致负载平方和更大，也不允许多撤换一对。
func TestPreservationBeatsBalance(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}},
		Reviewers: []Reviewer{
			{ID: "A", Department: "D1", BaseLoad: 2, MaxLoad: 9},
			{ID: "B", Department: "D2", BaseLoad: 2, MaxLoad: 9},
			{ID: "C", Department: "D3", BaseLoad: 2, MaxLoad: 9},
			{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 9},
			{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 9},
			{ID: "F", Department: "D3", BaseLoad: 0, MaxLoad: 9},
		},
		Previous: []Assignment{
			{MaterialID: "M1", ReviewerID: "A"},
			{MaterialID: "M1", ReviewerID: "B"},
			{MaterialID: "M1", ReviewerID: "C"},
		},
	}
	res := planAndCheck(t, in)
	// 保留全部 3 对：平方和 3*3^2+3*0^2=27；
	// 若改派 {D,E,F} 平方和仅 3，但需撤换 3 对，不被允许。
	if res.PreservedPairs != 3 {
		t.Fatalf("preserved = %d, want 3", res.PreservedPairs)
	}
	if res.LoadSquareSum != 27 {
		t.Fatalf("load square sum = %d, want 27", res.LoadSquareSum)
	}
}

// TestReplanCapacityShrink 重排时容量收紧，旧方案占用超额必须撤换，
// 且已有在办量不被旧方案重复计入。
func TestReplanCapacityShrink(t *testing.T) {
	reviewers := []Reviewer{
		{ID: "A", Department: "D1", BaseLoad: 1, MaxLoad: 2},
		{ID: "B", Department: "D2", BaseLoad: 1, MaxLoad: 2},
		{ID: "C", Department: "D3", BaseLoad: 1, MaxLoad: 2},
		{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 2},
		{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 2},
		{ID: "F", Department: "D3", BaseLoad: 0, MaxLoad: 2},
	}
	previous := []Assignment{
		{MaterialID: "M1", ReviewerID: "A"},
		{MaterialID: "M1", ReviewerID: "B"},
		{MaterialID: "M1", ReviewerID: "C"},
		{MaterialID: "M2", ReviewerID: "A"},
		{MaterialID: "M2", ReviewerID: "B"},
		{MaterialID: "M2", ReviewerID: "C"},
	}
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}},
		Reviewers: reviewers,
		Previous:  previous,
	}
	res := planAndCheck(t, in)
	// A/B/C 各只剩 1 个名额，旧方案让它们各评 2 份，必须撤换；
	// 最优保留 3 对（每材料留 1 个旧配对），D/E/F 补位。
	if res.PreservedPairs != 3 {
		t.Fatalf("preserved = %d, want 3", res.PreservedPairs)
	}
	// 负载：A,B,C 各 2，D,E,F 各 1 → 3*4+3*1=15。
	if res.LoadSquareSum != 15 {
		t.Fatalf("load square sum = %d, want 15", res.LoadSquareSum)
	}
}

// TestRandomCrossCheck 固定种子生成大量小实例，
// 将 Plan 的可行性与两个目标值同暴力枚举逐一核对。
func TestRandomCrossCheck(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	depts := []string{"D1", "D2", "D3", "D4"}
	for trial := 0; trial < 300; trial++ {
		nRev := 3 + rng.Intn(4)
		nMat := 1 + rng.Intn(3)
		in := Input{}
		for i := 0; i < nRev; i++ {
			in.Reviewers = append(in.Reviewers, Reviewer{
				ID:         fmt.Sprintf("R%d", i),
				Department: depts[rng.Intn(len(depts))],
				BaseLoad:   rng.Intn(3),
				MaxLoad:    1 + rng.Intn(4),
			})
		}
		for i := 0; i < nMat; i++ {
			m := Material{ID: fmt.Sprintf("M%d", i)}
			for _, r := range in.Reviewers {
				if rng.Intn(4) == 0 {
					m.Avoid = append(m.Avoid, r.ID)
				}
			}
			in.Materials = append(in.Materials, m)
		}
		// 随机锁定：先随机指定，再丢弃与回避冲突的，保证存在合法锁定子集。
		for _, m := range in.Materials {
			avoid := map[string]bool{}
			for _, id := range m.Avoid {
				avoid[id] = true
			}
			usedDept := map[string]bool{}
			for _, r := range in.Reviewers {
				if rng.Intn(6) != 0 || avoid[r.ID] || usedDept[r.Department] {
					continue
				}
				usedDept[r.Department] = true
				in.Locks = append(in.Locks, Lock{MaterialID: m.ID, ReviewerID: r.ID})
			}
		}
		// 随机旧方案：从合法配对中抽样（不要求自身合规）。
		for _, m := range in.Materials {
			for _, r := range in.Reviewers {
				if rng.Intn(3) == 0 {
					in.Previous = append(in.Previous, Assignment{MaterialID: m.ID, ReviewerID: r.ID})
				}
			}
		}

		res := Plan(in)
		feasible, preserved, square := bruteForceOptimum(in)
		if res.Feasible != feasible {
			t.Fatalf("trial %d: feasible=%v, brute force=%v\ninput=%+v", trial, res.Feasible, feasible, in)
		}
		if !res.Feasible {
			continue
		}
		if msg := checkValid(in, res); msg != "" {
			t.Fatalf("trial %d: invalid plan: %s\ninput=%+v", trial, msg, in)
		}
		if res.PreservedPairs != preserved || res.LoadSquareSum != square {
			t.Fatalf("trial %d: got (%d,%d), want (%d,%d)\ninput=%+v",
				trial, res.PreservedPairs, res.LoadSquareSum, preserved, square, in)
		}
	}
}

// TestInfeasibleByDepartment 总容量足够，但部门凑不齐 3 个不同部门而无解。
func TestInfeasibleByDepartment(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}},
		Reviewers: []Reviewer{
			{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 10},
			{ID: "B", Department: "D1", BaseLoad: 0, MaxLoad: 10},
			{ID: "C", Department: "D2", BaseLoad: 0, MaxLoad: 10},
			{ID: "D", Department: "D2", BaseLoad: 0, MaxLoad: 10},
		},
	}
	if res := Plan(in); res.Feasible {
		t.Fatalf("expected infeasible, got %v", res.Assignments)
	}
}

// TestInfeasibleByCapacity 总容量不足时无解，不能返回部分派评。
func TestInfeasibleByCapacity(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}},
		Reviewers: []Reviewer{
			{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 1},
			{ID: "B", Department: "D2", BaseLoad: 0, MaxLoad: 1},
			{ID: "C", Department: "D3", BaseLoad: 0, MaxLoad: 1},
			{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 1},
			{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 1},
		},
	}
	res := Plan(in)
	if res.Feasible {
		t.Fatalf("expected infeasible, got %v", res.Assignments)
	}
	if len(res.Assignments) != 0 {
		t.Fatalf("infeasible result must not carry partial assignments")
	}
}

// TestInfeasibleByLocks 各类锁定冲突均判无解。
func TestInfeasibleByLocks(t *testing.T) {
	reviewers := func() []Reviewer {
		return []Reviewer{
			{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 2},
			{ID: "B", Department: "D2", BaseLoad: 0, MaxLoad: 2},
			{ID: "C", Department: "D3", BaseLoad: 0, MaxLoad: 2},
			{ID: "D", Department: "D1", BaseLoad: 0, MaxLoad: 2},
			{ID: "E", Department: "D2", BaseLoad: 0, MaxLoad: 2},
			{ID: "F", Department: "D3", BaseLoad: 0, MaxLoad: 2},
		}
	}
	cases := map[string]Input{
		"锁定与回避冲突": {
			Materials: []Material{{ID: "M1", Avoid: []string{"A"}}},
			Reviewers: reviewers(),
			Locks:     []Lock{{MaterialID: "M1", ReviewerID: "A"}},
		},
		"同材料两条锁定同部门": {
			Materials: []Material{{ID: "M1"}},
			Reviewers: reviewers(),
			Locks: []Lock{
				{MaterialID: "M1", ReviewerID: "A"},
				{MaterialID: "M1", ReviewerID: "D"},
			},
		},
		"锁定超出评审员容量": {
			Materials: []Material{{ID: "M1"}, {ID: "M2"}, {ID: "M3"}},
			Reviewers: reviewers(),
			Locks: []Lock{
				{MaterialID: "M1", ReviewerID: "A"},
				{MaterialID: "M2", ReviewerID: "A"},
				{MaterialID: "M3", ReviewerID: "A"},
			},
		},
		"单材料锁定超过三席": {
			Materials: []Material{{ID: "M1"}},
			Reviewers: reviewers(),
			Locks: []Lock{
				{MaterialID: "M1", ReviewerID: "A"},
				{MaterialID: "M1", ReviewerID: "B"},
				{MaterialID: "M1", ReviewerID: "C"},
				{MaterialID: "M1", ReviewerID: "D"},
			},
		},
		"锁定引用未知评审员": {
			Materials: []Material{{ID: "M1"}},
			Reviewers: reviewers(),
			Locks:     []Lock{{MaterialID: "M1", ReviewerID: "ZZ"}},
		},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if res := Plan(in); res.Feasible {
				t.Fatalf("expected infeasible, got %v", res.Assignments)
			}
		})
	}
}

// TestFirstPlanBalancing 首次派评只追求负载平方和最小。
func TestFirstPlanBalancing(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}},
		Reviewers: []Reviewer{
			{ID: "A", Department: "D1", BaseLoad: 0, MaxLoad: 5},
			{ID: "B", Department: "D2", BaseLoad: 0, MaxLoad: 5},
			{ID: "C", Department: "D3", BaseLoad: 0, MaxLoad: 5},
			{ID: "E", Department: "D1", BaseLoad: 0, MaxLoad: 5},
			{ID: "F", Department: "D2", BaseLoad: 0, MaxLoad: 5},
			{ID: "G", Department: "D3", BaseLoad: 0, MaxLoad: 5},
		},
	}
	res := planAndCheck(t, in)
	if res.PreservedPairs != 0 {
		t.Fatalf("first plan should preserve nothing, got %d", res.PreservedPairs)
	}
	// 6 个名额均分给 6 人：平方和 6。
	if res.LoadSquareSum != 6 {
		t.Fatalf("load square sum = %d, want 6", res.LoadSquareSum)
	}
}
