// Package reviewplanner 提供质量材料评审派单规划能力。
//
// 上层调用方在进程内存中传入材料清单、评审员、回避名单、
// 人工锁定配对以及（重排时）上一版完整方案，本库在全部
// 合规完整方案中按两级目标求最优解并返回。
package reviewplanner

// Reviewer 描述一名评审员。
type Reviewer struct {
	ID         string // 评审员唯一标识
	Department string // 所属部门
	BaseLoad   int    // 已有在办量（仅统计本批次之外的任务）
	MaxLoad    int    // 总在办量上限（含本批次新增）
}

// Material 描述一份待评审材料。
type Material struct {
	ID    string   // 材料唯一标识
	Avoid []string // 回避名单：这些评审员不能评审本材料
}

// Lock 描述一对人工锁定的材料与评审员配对。
type Lock struct {
	MaterialID string
	ReviewerID string
}

// Assignment 描述一对材料与评审员的派评关系，占 1 个名额。
type Assignment struct {
	MaterialID string
	ReviewerID string
}

// Input 是一次规划的完整输入。
type Input struct {
	Materials []Material
	Reviewers []Reviewer
	Locks     []Lock
	// Previous 为重排时的上一版完整方案；首次派评传 nil。
	Previous []Assignment
}

// Result 是规划结果。
type Result struct {
	// Feasible 为 false 表示不存在合规完整方案，此时 Assignments 为空。
	Feasible    bool
	Assignments []Assignment

	// PreservedPairs 为最终方案中保留自 Previous 的配对数（第一目标值）。
	PreservedPairs int
	// LoadSquareSum 为所有评审员分配后总在办量的平方和（第二目标值）。
	LoadSquareSum int64
}
