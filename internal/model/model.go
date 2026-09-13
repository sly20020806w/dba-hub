package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// StringSlice 以 JSON 存 []string
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) { return json.Marshal(s) }
func (s *StringSlice) Scan(v interface{}) error {
	if v == nil {
		*s = nil
		return nil
	}
	switch b := v.(type) {
	case []byte:
		return json.Unmarshal(b, s)
	case string:
		return json.Unmarshal([]byte(b), s)
	}
	return errors.New("unsupported StringSlice")
}

// StringMap 以 JSON 存 map[string]string
type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) { return json.Marshal(m) }
func (m *StringMap) Scan(v interface{}) error {
	if v == nil {
		*m = nil
		return nil
	}
	switch b := v.(type) {
	case []byte:
		return json.Unmarshal(b, m)
	case string:
		return json.Unmarshal([]byte(b), m)
	}
	return errors.New("unsupported StringMap")
}

// DBInstance 被纳管的 MySQL 实例（课程 M2-14：RDS 作为资产纳管/绑定业务/标签/搜索）
type DBInstance struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"index" json:"name"`         // 实例名
	Env          string     `json:"env"`                      // prod/staging/test
	Host         string     `json:"host"`
	Port         int        `json:"port"`
	Username     string     `json:"username"`
	Password     string     `json:"password,omitempty"`
	ServiceNode  string     `json:"serviceNode"`              // 绑定的服务树节点
	Business     string     `json:"business"`                 // 所属业务
	Tags         StringSlice `gorm:"type:text" json:"tags"`
	Mode         string     `json:"mode"`                     // demo | mysql，demo 返回内置样例数据
	Version      string     `json:"version"`
	Status       string     `json:"status"`                   // ok / down / unknown
	LastCheckAt  *time.Time `json:"lastCheckAt"`
	LastScore    int        `json:"lastScore"`                // 最近一次健康分
	Remark       string     `json:"remark"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// CheckItem 巡检单项结果
type CheckItem struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Level  string `json:"level"`  // ok / warn / crit
	Value  string `json:"value"`
	Advice string `json:"advice"`
}

// InspectionRecord 一次巡检快照
type InspectionRecord struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	InstanceID uint        `gorm:"index" json:"instanceId"`
	Score      int         `json:"score"`
	Status     string      `json:"status"` // healthy/warn/critical
	ItemsJSON  string      `gorm:"type:text" json:"itemsJson"`
	Items      []CheckItem `gorm:"-" json:"items"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// SlowQueryStat 慢查询 digest 聚合快照（来自 performance_schema）
type SlowQueryStat struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	InstanceID     uint      `gorm:"index" json:"instanceId"`
	Digest         string    `gorm:"index" json:"digest"`
	Schema         string    `json:"schema"`
	SampleSQL      string    `gorm:"type:text" json:"sampleSql"`
	ExecCount      int64     `json:"execCount"`
	AvgLatencyMS   float64   `json:"avgLatencyMs"`
	MaxLatencyMS   float64   `json:"maxLatencyMs"`
	TotalLatencyS  float64   `json:"totalLatencyS"`
	AvgRowsExamined float64  `json:"avgRowsExamined"`
	AvgRowsSent     float64  `json:"avgRowsSent"`
	FullScan       bool      `json:"fullScan"`
	CreatedAt      time.Time `json:"createdAt"`
}

// TableStat 表空间/容量快照
type TableStat struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	InstanceID   uint      `gorm:"index:idx_inst_tbl,unique" json:"instanceId"`
	SchemaName   string    `gorm:"index:idx_inst_tbl,unique" json:"schemaName"`
	TableName    string    `gorm:"index:idx_inst_tbl,unique" json:"tableName"`
	Engine       string    `json:"engine"`
	TableRows    int64     `json:"tableRows"`
	DataMB       float64   `json:"dataMb"`
	IndexMB      float64   `json:"indexMb"`
	FragmentPct  float64   `json:"fragmentPct"` // data_free / (data+index)
	HasPrimary   bool      `json:"hasPrimary"`
	CreatedAt    time.Time `json:"createdAt"`
}

// QueryLog SQL 工作台执行记录（审计）
type QueryLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InstanceID uint      `gorm:"index" json:"instanceId"`
	Operator   string    `json:"operator"`
	SQLText    string    `gorm:"type:text" json:"sqlText"`
	ReadOnly   bool      `json:"readOnly"`
	Allowed    bool      `json:"allowed"`
	Reason     string    `json:"reason"`
	Rows       int       `json:"rows"`
	CostMS     int64     `json:"costMs"`
	CreatedAt  time.Time `json:"createdAt"`
}

func AllModels() []interface{} {
	return []interface{}{
		&DBInstance{}, &InspectionRecord{}, &SlowQueryStat{}, &TableStat{}, &QueryLog{},
	}
}
