package mysqlx

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"dba-hub/internal/model"
)

// demoProber 不连真实数据库，返回内置样例数据，用于无 MySQL 环境下的演示与前端联调。
// 所有数据在 UI/接口语义上与真实探测一致，实例 Mode=demo 时启用。
type demoProber struct {
	ins model.DBInstance
}

func newDemoProber(ins model.DBInstance) *demoProber { return &demoProber{ins: ins} }

func (d *demoProber) Close() {}
func (d *demoProber) Ping() error { return nil }

func (d *demoProber) Health() (*HealthInfo, error) {
	return &HealthInfo{
		Version: "8.0.36-demo", UptimeS: 86400 * 13,
		QPS: 1284.5, TPS: 96.2,
		ThreadsConnected: 312, ThreadsRunning: 18, MaxConnections: 500, ConnUsagePct: 62.4,
		AbortedConnects: 7, AbortedClients: 23,
		BufferPoolHitPct: 99.94, SlowQueries: 1482, CreatedTmpDisk: 96, SortMergePasses: 34,
		InnodbRowRead: 1820000000, InnodbRowInserted: 8400000, InnodbRowUpdated: 2100000, InnodbRowDeleted: 360000,
		BytesReceivedPerS: 84210, BytesSentPerS: 312040, CollectedAt: time.Now(),
	}, nil
}

func (d *demoProber) Replication() (*ReplicationInfo, error) {
	return &ReplicationInfo{Enabled: true, IORunning: "Yes", SQLRunning: "Yes",
		SecondsBehindMaster: 6, MasterHost: "10.0.0.21", AutoPosition: true}, nil
}

func (d *demoProber) Sessions() ([]SessionInfo, error) {
	return []SessionInfo{
		{ID: 8421, User: "order_rw", Host: "10.2.0.11:41022", DB: "order_db", Command: "Query", TimeS: 47, State: "Sending data", Info: "SELECT * FROM orders WHERE status=1 ORDER BY create_time DESC"},
		{ID: 8390, User: "app_rw", Host: "10.2.0.12:50211", DB: "order_db", Command: "Sleep", TimeS: 128, State: "", Info: ""},
		{ID: 8377, User: "repl", Host: "10.0.0.21:33061", DB: "", Command: "Binlog Dump GTID", TimeS: 1120000, State: "Source has sent all binlog", Info: ""},
		{ID: 8355, User: "dba_ro", Host: "10.2.0.9:33520", DB: "order_db", Command: "Query", TimeS: 3, State: "executing", Info: "SHOW FULL PROCESSLIST"},
	}, nil
}

func (d *demoProber) Kill(id int64) error { return nil }

func (d *demoProber) LongTransactions() ([]LongTxn, error) {
	return []LongTxn{
		{TrxID: "2814749767", State: "RUNNING", StartedS: 412, RowsLocked: 18, RowsModified: 18,
			Query: "UPDATE order_items SET status=2 WHERE order_id=20260913001"},
	}, nil
}

func (d *demoProber) Tables() ([]SchemaTableInfo, error) {
	return []SchemaTableInfo{
		{SchemaName: "order_db", TableName: "orders", Engine: "InnoDB", TableRows: 48200000, DataMB: 14820.5, IndexMB: 9210.2, FreeMB: 120.4, HasPrimary: true},
		{SchemaName: "order_db", TableName: "order_items", Engine: "InnoDB", TableRows: 186000000, DataMB: 38400.0, IndexMB: 21200.0, FreeMB: 5400.0, HasPrimary: true},
		{SchemaName: "order_db", TableName: "tmp_import_log", Engine: "InnoDB", TableRows: 320000, DataMB: 96.2, IndexMB: 0, FreeMB: 61.8, HasPrimary: false},
		{SchemaName: "order_db", TableName: "payments", Engine: "InnoDB", TableRows: 39100000, DataMB: 9800.1, IndexMB: 6100.4, FreeMB: 88.0, HasPrimary: true},
		{SchemaName: "user_db", TableName: "users", Engine: "InnoDB", TableRows: 2100000, DataMB: 480.6, IndexMB: 320.1, FreeMB: 12.2, HasPrimary: true},
	}, nil
}

func (d *demoProber) SlowTop(n int) ([]SlowDigest, error) {
	all := []SlowDigest{
		{Schema: "order_db", SampleSQL: "SELECT * FROM orders WHERE status = ? ORDER BY create_time DESC", ExecCount: 18420,
			AvgLatencyMS: 842.5, MaxLatencyMS: 4210.0, TotalLatencyS: 15518.9, AvgRowsExamined: 48200000, AvgRowsSent: 20, FullScan: true},
		{Schema: "order_db", SampleSQL: "SELECT count(*) FROM order_items WHERE order_id = ?", ExecCount: 96500,
			AvgLatencyMS: 210.3, MaxLatencyMS: 1800.0, TotalLatencyS: 20293.9, AvgRowsExamined: 186000000, AvgRowsSent: 1, FullScan: true},
		{Schema: "order_db", SampleSQL: "UPDATE payments SET state=? WHERE pay_no=?", ExecCount: 41200,
			AvgLatencyMS: 38.2, MaxLatencyMS: 620.0, TotalLatencyS: 1573.8, AvgRowsExamined: 1, AvgRowsSent: 0, FullScan: false},
		{Schema: "user_db", SampleSQL: "SELECT * FROM users WHERE phone=?", ExecCount: 220000,
			AvgLatencyMS: 4.1, MaxLatencyMS: 90.0, TotalLatencyS: 902.0, AvgRowsExamined: 1, AvgRowsSent: 1, FullScan: false},
	}
	if n > 0 && n < len(all) {
		return all[:n], nil
	}
	return all, nil
}

func (d *demoProber) Query(sqlText string, maxRows int) (*QueryResult, error) {
	low := strings.ToLower(sqlText)
	start := time.Now()
	if strings.Contains(low, "processlist") {
		s, _ := d.Sessions()
		res := &QueryResult{Columns: []string{"id", "user", "db", "command", "time", "info"}}
		for _, x := range s {
			res.Rows = append(res.Rows, []string{fmt.Sprint(x.ID), x.User, x.DB, x.Command, fmt.Sprint(x.TimeS), x.Info})
		}
		res.CostMS = time.Since(start).Milliseconds()
		return res, nil
	}
	return &QueryResult{
		Columns: []string{"id", "name", "value", "note"},
		Rows: [][]string{
			{"1", "demo_row_a", "100", "内置演示数据，配置真实 MySQL 实例后返回真实结果"},
			{"2", "demo_row_b", "200", ""},
		},
		CostMS: 2,
	}, nil
}

func (d *demoProber) Explain(sqlText string) (*QueryResult, error) {
	low := strings.ToLower(sqlText)
	// 等值匹配（=值 / = ?）视为走索引；否则演示为全表扫描，便于讲“索引缺失”
	eq := regexp.MustCompile(`=\s*(\?|\d|')`)
	usesIndex := strings.Contains(low, "where") && eq.MatchString(sqlText)
	if usesIndex {
		return &QueryResult{Columns: []string{"id", "select_type", "table", "type", "key", "rows", "Extra"}, Rows: [][]string{
			{"1", "SIMPLE", "orders", "const", "PRIMARY", "1", "Using index condition"},
		}, CostMS: 1}, nil
	}
	return &QueryResult{Columns: []string{"id", "select_type", "table", "type", "key", "rows", "Extra"}, Rows: [][]string{
		{"1", "SIMPLE", "orders", "ALL", "NULL", "48200000", "Using where; Using filesort"},
	}, CostMS: 1}, nil
}
