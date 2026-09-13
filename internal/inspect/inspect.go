package inspect

import (
	"fmt"
	"sort"

	"dba-hub/internal/model"
	"dba-hub/internal/mysqlx"
)

// Report 一次完整巡检报告
type Report struct {
	InstanceID       uint                       `json:"instanceId"`
	Score            int                        `json:"score"`
	Status           string                     `json:"status"`
	Health           *mysqlx.HealthInfo         `json:"health"`
	Replication      *mysqlx.ReplicationInfo    `json:"replication"`
	Items            []model.CheckItem          `json:"items"`
	BigTables        []mysqlx.SchemaTableInfo   `json:"bigTables"`
	NoPKTables       []mysqlx.SchemaTableInfo   `json:"noPkTables"`
	FragmentedTables []mysqlx.SchemaTableInfo   `json:"fragmentedTables"`
	LongTxns         []mysqlx.LongTxn           `json:"longTxns"`
}

type scorer struct {
	items []model.CheckItem
	score int
}

func (s *scorer) add(key, name, level, value, advice string) {
	s.items = append(s.items, model.CheckItem{Key: key, Name: name, Level: level, Value: value, Advice: advice})
	switch level {
	case "crit":
		s.score -= 15
	case "warn":
		s.score -= 6
	}
}

// Run 对一个已连接的 Prober 执行全项巡检
func Run(instanceID uint, p mysqlx.Prober) (*Report, error) {
	h, err := p.Health()
	if err != nil {
		return nil, err
	}
	rep, _ := p.Replication()
	tables, _ := p.Tables()
	longTxns, _ := p.LongTransactions()

	s := &scorer{score: 100}

	// 1. 连接数使用率
	switch {
	case h.ConnUsagePct >= 85:
		s.add("conn_usage", "连接数使用率", "crit", fmt.Sprintf("%.1f%%", h.ConnUsagePct), "接近 max_connections，调大上限或排查连接泄漏/慢查询堆积")
	case h.ConnUsagePct >= 70:
		s.add("conn_usage", "连接数使用率", "warn", fmt.Sprintf("%.1f%%", h.ConnUsagePct), "关注连接增长趋势，检查应用连接池配置")
	default:
		s.add("conn_usage", "连接数使用率", "ok", fmt.Sprintf("%.1f%% (%d/%d)", h.ConnUsagePct, h.ThreadsConnected, h.MaxConnections), "")
	}

	// 2. 缓冲池命中率
	switch {
	case h.BufferPoolHitPct < 95:
		s.add("bp_hit", "InnoDB 缓冲池命中率", "crit", fmt.Sprintf("%.2f%%", h.BufferPoolHitPct), "命中率过低，增大 innodb_buffer_pool_size 或排查全表扫描")
	case h.BufferPoolHitPct < 99:
		s.add("bp_hit", "InnoDB 缓冲池命中率", "warn", fmt.Sprintf("%.2f%%", h.BufferPoolHitPct), "命中率偏低，建议保持在 99% 以上")
	default:
		s.add("bp_hit", "InnoDB 缓冲池命中率", "ok", fmt.Sprintf("%.2f%%", h.BufferPoolHitPct), "")
	}

	// 3. 运行中线程（饱和度）
	if h.ThreadsRunning >= 32 {
		s.add("threads_running", "运行中线程数", "crit", fmt.Sprintf("%d", h.ThreadsRunning), "并发执行线程过多，通常是慢查询堆积，结合慢查询与 processlist 定位")
	} else if h.ThreadsRunning >= 16 {
		s.add("threads_running", "运行中线程数", "warn", fmt.Sprintf("%d", h.ThreadsRunning), "关注峰值并发")
	} else {
		s.add("threads_running", "运行中线程数", "ok", fmt.Sprintf("%d", h.ThreadsRunning), "")
	}

	// 4. 磁盘临时表 / 排序合并
	if h.CreatedTmpDisk > 1000 {
		s.add("tmp_disk", "落盘临时表(累计)", "warn", fmt.Sprintf("%d", h.CreatedTmpDisk), "排序/分组落盘，检查 tmp_table_size/max_heap_table_size 与索引")
	} else {
		s.add("tmp_disk", "落盘临时表(累计)", "ok", fmt.Sprintf("%d", h.CreatedTmpDisk), "")
	}
	if h.SortMergePasses > 100 {
		s.add("sort_merge", "排序合并次数(累计)", "warn", fmt.Sprintf("%d", h.SortMergePasses), "排序缓冲不足，适当增大 sort_buffer_size 或补索引")
	} else {
		s.add("sort_merge", "排序合并次数(累计)", "ok", fmt.Sprintf("%d", h.SortMergePasses), "")
	}

	// 5. 中断连接
	if h.AbortedConnects > 100 {
		s.add("aborted", "失败/中断连接(累计)", "warn", fmt.Sprintf("connects=%d clients=%d", h.AbortedConnects, h.AbortedClients), "排查网络、账号白名单或应用是否异常断连")
	} else {
		s.add("aborted", "失败/中断连接(累计)", "ok", fmt.Sprintf("connects=%d clients=%d", h.AbortedConnects, h.AbortedClients), "")
	}

	// 6. 主从复制
	if rep != nil && rep.Enabled {
		switch {
		case rep.IORunning != "Yes" || rep.SQLRunning != "Yes":
			s.add("replication", "主从复制线程", "crit", fmt.Sprintf("IO=%s SQL=%s", rep.IORunning, rep.SQLRunning), "复制线程停止，立即查看复制报错并修复")
		case rep.SecondsBehindMaster >= 60:
			s.add("replication", "主从延迟", "crit", fmt.Sprintf("%ds", rep.SecondsBehindMaster), "延迟过大，排查大事务、从库负载与网络")
		case rep.SecondsBehindMaster >= 10:
			s.add("replication", "主从延迟", "warn", fmt.Sprintf("%ds", rep.SecondsBehindMaster), "关注延迟趋势")
		default:
			s.add("replication", "主从复制", "ok", fmt.Sprintf("正常, 延迟%ds", rep.SecondsBehindMaster), "")
		}
	}

	// 7. 长事务
	var maxTxnS int64
	for _, t := range longTxns {
		if t.StartedS > maxTxnS {
			maxTxnS = t.StartedS
		}
	}
	switch {
	case maxTxnS >= 300:
		s.add("long_txn", "最长事务时长", "crit", fmt.Sprintf("%ds", maxTxnS), "存在超过5分钟长事务，会撑大 undo、阻塞 DDL/锁等待，及时处理")
	case maxTxnS >= 60:
		s.add("long_txn", "最长事务时长", "warn", fmt.Sprintf("%ds", maxTxnS), "存在较长事务，确认是否预期")
	default:
		s.add("long_txn", "最长事务时长", "ok", fmt.Sprintf("%ds", maxTxnS), "")
	}

	// 8. 无主键表 / 碎片 / 大表
	var noPK, frag []mysqlx.SchemaTableInfo
	for _, t := range tables {
		if !t.HasPrimary {
			noPK = append(noPK, t)
		}
		total := t.DataMB + t.IndexMB
		if total > 0 && t.FreeMB/total > 0.3 && t.FreeMB > 100 {
			frag = append(frag, t)
		}
	}
	if len(noPK) > 0 {
		s.add("no_pk", "无主键表", "warn", fmt.Sprintf("%d 张", len(noPK)), "无主键表在主从复制/在线变更时风险高，建议补主键")
	} else {
		s.add("no_pk", "无主键表", "ok", "0 张", "")
	}
	if len(frag) > 0 {
		s.add("fragment", "高碎片表", "warn", fmt.Sprintf("%d 张", len(frag)), "data_free 占比高，可在低峰期 OPTIMIZE/在线重建")
	} else {
		s.add("fragment", "高碎片表", "ok", "0 张", "")
	}

	big := append([]mysqlx.SchemaTableInfo{}, tables...)
	sort.Slice(big, func(i, j int) bool { return big[i].DataMB+big[i].IndexMB > big[j].DataMB+big[j].IndexMB })
	if len(big) > 10 {
		big = big[:10]
	}

	score := s.score
	if score < 0 {
		score = 0
	}
	status := "healthy"
	if score < 85 {
		status = "warn"
	}
	if score < 60 {
		status = "critical"
	}
	return &Report{
		InstanceID: instanceID, Score: score, Status: status, Items: s.items, Health: h,
		Replication: rep, BigTables: big, NoPKTables: noPK, FragmentedTables: frag, LongTxns: longTxns,
	}, nil
}
