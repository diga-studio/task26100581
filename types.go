// Package reviewplanner 提供质量材料评审派单规划能力。
// 输入与规划结果均保存在进程内存中，由上层直接调用。
package reviewplanner

// Reviewer 评审员。
type Reviewer struct {
	ID         string // 评审员唯一标识
	Department string // 所属部门
	Existing   int    // 已有在办量（仅统计本批次之外的任务）
	Capacity   int    // 总在办量上限（已有在办量 + 本批次分配量 不得超过该值）
}

// Material 待评审材料。
type Material struct {
	ID string // 材料唯一标识
}

// Lock 人工锁定配对：指定评审员必须评审对应材料。
type Lock struct {
	MaterialID string
	ReviewerID string
}

// Assignment 一条派评配对：一份材料与一名评审员。
type Assignment struct {
	MaterialID string
	ReviewerID string
}

// Input 规划输入。
type Input struct {
	Materials []Material   // 本批次材料清单
	Reviewers []Reviewer   // 评审员名单及所属部门、在办量与上限
	Avoid     []Assignment // 逐材料回避名单（ReviewerID 不能评审 MaterialID）
	Locks     []Lock       // 人工锁定配对
	Previous  []Assignment // 上次完整方案（重排时提供；首次派评传 nil）
}

// Result 规划结果。
type Result struct {
	Assignments []Assignment // 完整派评方案：每份材料恰好 3 名不同评审员
}

// Stats 方案目标值，便于上层核对。
type Stats struct {
	KeptPairs   int // 保留的旧方案配对数（第一目标，越大越好）
	LoadSquared int // 分配后总在办量平方和（第二目标，越小越好）
}
