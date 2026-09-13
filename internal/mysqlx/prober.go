package mysqlx

import (
	"fmt"
	"time"

	"dba-hub/internal/model"
)

// HealthInfo 实例健康/性能快照（对应 SHOW GLOBAL STATUS / VARIABLES）
type HealthInfo struct {
	Version            string  `json:"version"`
	UptimeS            int64   `json:"uptimeS"`
	QPS                float64 `json:"qps"`
	TPS                float64 `json:"tps"`
	ThreadsConnected   int64   `json:"threadsConnected"`
	ThreadsRunning     int64   `json:"threadsRunning"`
	MaxConnections     int64   `json:"maxConnections"`
	ConnUsagePct       float64 `json:"connUsagePct"`
	AbortedConnects    int64   `json:"abortedConnects"`
	AbortedClients     int64   `json:"abortedClients"`
	BufferPoolHitPct   float64 `json:"bufferPoolHitPct"`
	SlowQueries        int64   `json:"slowQueries"`
	CreatedTmpDisk     int64   `json:"createdTmpDiskTables"`
	SortMergePasses    int64   `json:"sortMergePasses"`
	InnodbRowRead      int64   `json:"innodbRowRead"`
	InnodbRowInserted  int64   `json:"innodbRowInserted"`
	InnodbRowUpdated   int64   `json:"innodbRowUpdated"`
	InnodbRowDeleted   int64   `json:"innodbRowDeleted"`
	BytesReceivedPerS float64 `json:"bytesRecvPerS"`
	BytesSentPerS     float64 `json:"bytesSentPerS"`
	CollectedAt       time.Time `json:"collectedAt"`
}

// ReplicationInfo 主从复制状态
type ReplicationInfo struct {
	Enabled            bool   `json:"enabled"`
	IORunning          string `json:"ioRunning"`
	SQLRunning         string `json:"sqlRunning"`
	SecondsBehindMaster int64 `json:"secondsBehindMaster"`
	MasterHost         string `json:"masterHost"`
	AutoPosition       bool   `json:"autoPosition"`
}

// SessionInfo 会话（processlist）
type SessionInfo struct {
	ID      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	TimeS   int64  `json:"timeS"`
	State   string `json:"state"`
	Info    string `json:"info"`
}

// LongTxn 长事务（information_schema.innodb_trx）
type LongTxn struct {
	TrxID      string `json:"trxId"`
	State      string `json:"state"`
	StartedS   int64  `json:"startedS"`
	RowsLocked int64  `json:"rowsLocked"`
	RowsModified int64 `json:"rowsModified"`
	Query      string `json:"query"`
}

// SchemaTableInfo 库表容量
type SchemaTableInfo struct {
	SchemaName string  `json:"schemaName"`
	TableName  string  `json:"tableName"`
	Engine     string  `json:"engine"`
	TableRows  int64   `json:"tableRows"`
	DataMB     float64 `json:"dataMb"`
	IndexMB    float64 `json:"indexMb"`
	FreeMB     float64 `json:"freeMb"`
	HasPrimary bool    `json:"hasPrimary"`
}

// SlowDigest 慢查询 digest 聚合
type SlowDigest struct {
	Schema          string  `json:"schema"`
	SampleSQL       string  `json:"sampleSql"`
	ExecCount       int64   `json:"execCount"`
	AvgLatencyMS    float64 `json:"avgLatencyMs"`
	MaxLatencyMS    float64 `json:"maxLatencyMs"`
	TotalLatencyS   float64 `json:"totalLatencyS"`
	AvgRowsExamined float64 `json:"avgRowsExamined"`
	AvgRowsSent     float64 `json:"avgRowsSent"`
	FullScan        bool    `json:"fullScan"`
}

// QueryResult 只读查询结果
type QueryResult struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	CostMS  int64      `json:"costMs"`
}

// Prober 目标实例探测抽象：真实 MySQL 与内置 demo 实现同一接口
type Prober interface {
	Ping() error
	Health() (*HealthInfo, error)
	Replication() (*ReplicationInfo, error)
	Sessions() ([]SessionInfo, error)
	Kill(id int64) error
	LongTransactions() ([]LongTxn, error)
	Tables() ([]SchemaTableInfo, error)
	SlowTop(n int) ([]SlowDigest, error)
	Query(sqlText string, maxRows int) (*QueryResult, error)
	Explain(sqlText string) (*QueryResult, error)
	Close()
}

// NewProber 按实例模式返回探测器
func NewProber(ins model.DBInstance) (Prober, error) {
	if ins.Mode == "demo" {
		return newDemoProber(ins), nil
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&timeout=4s&readTimeout=8s&writeTimeout=8s",
		ins.Username, ins.Password, ins.Host, ins.Port)
	return newMysqlProber(dsn)
}
