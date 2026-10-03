package reviewplanner

import (
	"errors"
	"testing"
)

// 场景一：候选交错且含人工锁定，锁定挤占容量后，
// 其他材料必须相应调整才能填满全部席位。
func TestFirstPlanInterleavedWithLocks(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}, {ID: "M3"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 0, Capacity: 1},
			{ID: "A2", Department: "A", Existing: 0, Capacity: 2},
			{ID: "B1", Department: "B", Existing: 0, Capacity: 2},
			{ID: "B2", Department: "B", Existing: 0, Capacity: 1},
			{ID: "C1", Department: "C", Existing: 0, Capacity: 2},
			{ID: "C2", Department: "C", Existing: 0, Capacity: 1},
		},
		// M1 回避 B2、C2，再锁定 A1：M1 被唯一锁定为 {A1,B1,C1}，
		// 占满 A1/B1/C1 的部分容量，M2、M3 只能在余量中交错分配。
		Avoid: []Assignment{
			{MaterialID: "M1", ReviewerID: "B2"},
			{MaterialID: "M1", ReviewerID: "C2"},
		},
		Locks: []Lock{{MaterialID: "M1", ReviewerID: "A1"}},
	}
	res, stats, err := Plan(in)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	checkValid(t, in, res)
	if stats.KeptPairs != 0 {
		t.Errorf("KeptPairs = %d, want 0 (no previous plan)", stats.KeptPairs)
	}
	// 与独立暴力枚举核对第二目标。
	_, wantLoad2, feasible := bruteForce(in)
	if !feasible {
		t.Fatal("brute force says infeasible")
	}
	if stats.LoadSquared != wantLoad2 {
		t.Errorf("LoadSquared = %d, want %d (brute force)", stats.LoadSquared, wantLoad2)
	}
	// M1 的锁定组合必须原样出现。
	got := map[string]bool{}
	for _, as := range res.Assignments {
		if as.MaterialID == "M1" {
			got[as.ReviewerID] = true
		}
	}
	for _, want := range []string{"A1", "B1", "C1"} {
		if !got[want] {
			t.Errorf("M1 missing expected reviewer %s, got %v", want, got)
		}
	}
}

// 场景二：总容量足够，但部门人数结构使三部门约束无法满足。
func TestInfeasibleByDepartment(t *testing.T) {
	in := Input{
		Materials: []Material{{ID: "M1"}, {ID: "M2"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 0, Capacity: 5},
			{ID: "A2", Department: "A", Existing: 0, Capacity: 5},
			{ID: "B1", Department: "B", Existing: 0, Capacity: 5},
			{ID: "C1", Department: "C", Existing: 0, Capacity: 1}, // C 部门总容量只有 1
		},
	}
	// 总容量 16 >= 6，但每份材料都需要一名 C 部门评审员，C 只能覆盖 1 份。
	_, _, err := Plan(in)
	if !errors.Is(err, ErrNoSolution) {
		t.Fatalf("err = %v, want ErrNoSolution", err)
	}
	if _, _, feasible := bruteForce(in); feasible {
		t.Fatal("brute force disagrees: feasible")
	}
}

// 场景三：总容量足够，但锁定冲突导致无解。
func TestInfeasibleByLockConflict(t *testing.T) {
	base := Input{
		Materials: []Material{{ID: "M1"}},
		Reviewers: []Reviewer{
			{ID: "A1", Department: "A", Existing: 0, Capacity: 5},
			{ID: "B1", Department: "B", Existing: 0, Capacity: 5},
			{ID: "C1", Department: "C", Existing: 0, Capacity: 5},
		},
	}

	// 锁定与回避冲突。
	in := base
	in.Avoid = []Assignment{{MaterialID: "M1", ReviewerID: "A1"}}
	in.Locks = []Lock{{MaterialID: "M1", ReviewerID: "A1"}}
	if _, _, err := Plan(in); !errors.Is(err, ErrNoSolution) {
		t.Errorf("lock vs avoid: err = %v, want ErrNoSolution", err)
	}

	// 同一材料锁定两名同部门评审员。
	in = base
	in.Avoid = nil
	in.Reviewers = append([]Reviewer{}, base.Reviewers...)
	in.Reviewers = append(in.Reviewers, Reviewer{ID: "A2", Department: "A", Existing: 0, Capacity: 5})
	in.Locks = []Lock{
		{MaterialID: "M1", ReviewerID: "A1"},
		{MaterialID: "M1", ReviewerID: "A2"},
	}
	if _, _, err := Plan(in); !errors.Is(err, ErrNoSolution) {
		t.Errorf("same-department locks: err = %v, want ErrNoSolution", err)
	}

	// 锁定的评审员容量不足。
	in = base
	in.Reviewers = append([]Reviewer{}, base.Reviewers...)
	in.Reviewers[0].Existing = 5 // A1 已满
	in.Locks = []Lock{{MaterialID: "M1", ReviewerID: "A1"}}
	if _, _, err := Plan(in); !errors.Is(err, ErrNoSolution) {
		t.Errorf("locked reviewer at capacity: err = %v, want ErrNoSolution", err)
	}
}
