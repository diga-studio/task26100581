# reviewplanner

质量材料评审派单规划库（Go）。输入与规划结果均在进程内存中，由上层直接调用 `Plan`。

## 功能

- 首次自动派评与基于上次方案的重排（同一批材料）。
- 每份材料恰好分配 3 名不同评审员，且来自 3 个不同部门。
- 支持逐材料回避名单、人工锁定配对（可只锁定部分席位）、每人已有在办量与总上限。
- 目标：先最大化保留旧方案配对数，再最小化分配后总在办量平方和；第一目标绝对优先。
- 存在合规完整方案则返回之，否则返回 `ErrNoSolution`，绝不返回部分派评。

## 使用

```go
in := reviewplanner.Input{
    Materials: []reviewplanner.Material{{ID: "M1"}, {ID: "M2"}},
    Reviewers: []reviewplanner.Reviewer{
        {ID: "A1", Department: "A", Existing: 1, Capacity: 6},
        // ...
    },
    Avoid:    []reviewplanner.Assignment{{MaterialID: "M1", ReviewerID: "A1"}}, // 回避名单
    Locks:    []reviewplanner.Lock{{MaterialID: "M2", ReviewerID: "B1"}},       // 人工锁定
    Previous: prev.Assignments, // 重排时传入上次完整方案；首次派评传 nil
}
res, stats, err := reviewplanner.Plan(in)
if errors.Is(err, reviewplanner.ErrNoSolution) {
    // 不存在合规完整方案
}
// res.Assignments：完整派评方案
// stats.KeptPairs / stats.LoadSquared：两个目标的取值
```

说明：

- `Existing` 只统计本批次之外的在办量；重排时旧方案不会重复计入。
- 重排时材料集合、评审员名单及部门须保持不变；容量、回避名单、锁定可变化。
- 锁定不能绕过回避、部门或容量限制，冲突时整体无解。
- 回避/锁定引用不存在的材料或评审员会返回参数错误。

## 测试

```sh
go test ./...
```

测试覆盖：交错候选与人工锁定的首次派评、部门限制无解、锁定冲突无解、
新增回避后的多材料级联重排、最少撤换优先于负载均衡；小批次场景的两个
目标值均与测试内独立暴力枚举的最优值核对。
