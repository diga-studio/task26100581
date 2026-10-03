package reviewplanner

import "testing"

// 场景四：旧方案新增回避关系后，需要连带调整多份材料才能恢复合规。
// 两个目标值均与独立暴力枚举的最优值核对。
func TestReplanCascadeAfterNewAvoid(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}, {ID: "M3"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 0, Capacity: 2},
			{ID: "A2", Department: "A", Existing: 0, Capacity: 2},
			{ID: "B1", Department: "B", Existing: 0, Capacity: 2},
			{ID: "B2", Department: "B", Existing: 0, Capacity: 1},
			{ID: "C1", Department: "C", Existing: 0, Capacity: 1},
			{ID: "C2", Department: "C", Existing: 0, Capacity: 2},
		},
		Previous: []Assignment{
			{MaterialID: "M1", ReviewerID: "A1"}, {MaterialID: "M1", ReviewerID: "B1"}, {MaterialID: "M1", ReviewerID: "C1"},
			{MaterialID: "M2", ReviewerID: "A2"}, {MaterialID: "M2", ReviewerID: "B2"}, {MaterialID: "M2", ReviewerID: "C2"},
			{MaterialID: "M3", ReviewerID: "A1"}, {MaterialID: "M3", ReviewerID: "B1"}, {MaterialID: "M3", ReviewerID: "C2"},
		},
		// 新增回避：M3 回避 A1 与 B1。M3 只能改用 A2 + B2，
		// 但 B2 容量 1 已被 M2 占用，A2 容量也会超限，
		// 迫使 M1/M2 连带调整。
		Avoid: []Assignment{
			{MaterialID: "M3", ReviewerID: "A1"},
			{MaterialID: "M3", ReviewerID: "B1"},
		},
	}
	res, stats, err := Plan(in)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	checkValid(t, in, res)

	wantKept, wantLoad2, feasible := bruteForce(in)
	if !feasible {
		t.Fatal("brute force says infeasible")
	}
	if stats.KeptPairs != wantKept {
		t.Errorf("KeptPairs = %d, want %d (brute force)", stats.KeptPairs, wantKept)
	}
	if stats.LoadSquared != wantLoad2 {
		t.Errorf("LoadSquared = %d, want %d (brute force)", stats.LoadSquared, wantLoad2)
	}
	if got := keptPairs(in, res); got != stats.KeptPairs {
		t.Errorf("result keeps %d pairs, stats say %d", got, stats.KeptPairs)
	}
	// 旧方案 9 条配对中至少 M3 的 2 条因新回避必然撤换。
	if stats.KeptPairs > 7 {
		t.Errorf("KeptPairs = %d, impossible: M3 lost 2 pairs to new avoid", stats.KeptPairs)
	}
	t.Logf("kept %d/9 pairs, load squared %d", stats.KeptPairs, stats.LoadSquared)
}

// 场景五：最少撤换绝对优先于负载均衡。
// 保留全部 3 条旧配对的方案负载更不均衡，但必须被选中。
func TestMinDisruptionBeatsBalance(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 5, Capacity: 10},
			{ID: "B1", Department: "B", Existing: 5, Capacity: 10},
			{ID: "C1", Department: "C", Existing: 5, Capacity: 10},
			{ID: "C2", Department: "C", Existing: 0, Capacity: 10},
		},
		Previous: []Assignment{
			{MaterialID: "M1", ReviewerID: "A1"},
			{MaterialID: "M1", ReviewerID: "B1"},
			{MaterialID: "M1", ReviewerID: "C1"},
		},
	}
	res, stats, err := Plan(in)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	checkValid(t, in, res)
	// 全保留：负载平方和 6^2*3 = 108；
	// 撤换 C1->C2 更均衡：6^2*2+1 = 73，但少保留 1 条，不允许。
	if stats.KeptPairs != 3 {
		t.Errorf("KeptPairs = %d, want 3 (disruption minimization wins)", stats.KeptPairs)
	}
	if stats.LoadSquared != 108 {
		t.Errorf("LoadSquared = %d, want 108", stats.LoadSquared)
	}
	wantKept, wantLoad2, feasible := bruteForce(in)
	if !feasible || stats.KeptPairs != wantKept || stats.LoadSquared != wantLoad2 {
		t.Errorf("mismatch with brute force: got (%d,%d), want (%d,%d)",
			stats.KeptPairs, stats.LoadSquared, wantKept, wantLoad2)
	}
}

// 场景六：首次派评只考虑负载均衡，与暴力枚举核对。
func TestFirstPlanBalancesLoad(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 1, Capacity: 6},
			{ID: "A2", Department: "A", Existing: 3, Capacity: 6},
			{ID: "B1", Department: "B", Existing: 0, Capacity: 6},
			{ID: "B2", Department: "B", Existing: 2, Capacity: 6},
			{ID: "C1", Department: "C", Existing: 4, Capacity: 6},
			{ID: "C2", Department: "C", Existing: 1, Capacity: 6},
		},
	}
	res, stats, err := Plan(in)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	checkValid(t, in, res)
	_, wantLoad2, feasible := bruteForce(in)
	if !feasible {
		t.Fatal("brute force says infeasible")
	}
	if stats.LoadSquared != wantLoad2 {
		t.Errorf("LoadSquared = %d, want %d (brute force)", stats.LoadSquared, wantLoad2)
	}
}
