package reviewplanner

import "container/list"

// flowEdge 是最小费用流网络中的一条边。
type flowEdge struct {
	to      int
	rev     int   // 反向边在邻接表中的下标
	cap     int   // 剩余容量
	cost    int64 // 单位流量费用
	pairIdx int   // 仅“材料-部门 -> 评审员”边有效：对应 candidatePairs 下标，-1 表示非配对面
}

// minCostFlow 支持负费用边的最小费用最大流。
type minCostFlow struct {
	graph [][]flowEdge
}

func newMinCostFlow(n int) *minCostFlow {
	return &minCostFlow{graph: make([][]flowEdge, n)}
}

func (m *minCostFlow) addEdge(from, to, cap int, cost int64, pairIdx int) {
	fwd := flowEdge{to: to, rev: len(m.graph[to]), cap: cap, cost: cost, pairIdx: pairIdx}
	bwd := flowEdge{to: from, rev: len(m.graph[from]), cap: 0, cost: -cost, pairIdx: -1}
	m.graph[from] = append(m.graph[from], fwd)
	m.graph[to] = append(m.graph[to], bwd)
}

// augment 沿 source->sink 逐单位增广，返回 (流量, 费用)。
// 每次用 SPFA 找最小费用增广路，可正确处理负费用边。
func (m *minCostFlow) augment(source, sink, maxFlow int) (int, int64) {
	n := len(m.graph)
	flow := 0
	var cost int64
	for flow < maxFlow {
		const inf = int64(1) << 62
		dist := make([]int64, n)
		inQueue := make([]bool, n)
		prevNode := make([]int, n)
		prevEdge := make([]int, n)
		for i := range dist {
			dist[i] = inf
		}
		dist[source] = 0
		q := list.New()
		q.PushBack(source)
		inQueue[source] = true
		for q.Len() > 0 {
			u := q.Remove(q.Front()).(int)
			inQueue[u] = false
			for ei, e := range m.graph[u] {
				if e.cap <= 0 {
					continue
				}
				if nd := dist[u] + e.cost; nd < dist[e.to] {
					dist[e.to] = nd
					prevNode[e.to] = u
					prevEdge[e.to] = ei
					if !inQueue[e.to] {
						q.PushBack(e.to)
						inQueue[e.to] = true
					}
				}
			}
		}
		if dist[sink] == inf {
			break
		}
		for v := sink; v != source; v = prevNode[v] {
			u, ei := prevNode[v], prevEdge[v]
			rev := m.graph[u][ei].rev
			m.graph[u][ei].cap--
			m.graph[v][rev].cap++
		}
		flow++
		cost += dist[sink]
	}
	return flow, cost
}
