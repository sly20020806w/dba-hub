package mysqlx

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlProber struct {
	db *sql.DB
}

func newMysqlProber(dsn string) (*mysqlProber, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(30 * time.Second)
	return &mysqlProber{db: db}, nil
}

func (m *mysqlProber) Close() { _ = m.db.Close() }

func (m *mysqlProber) Ping() error { return m.db.Ping() }

func atoi(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}
func ftoi(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func (m *mysqlProber) kv(sqlText string) (map[string]string, error) {
	rows, err := m.db.Query(sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		out[k] = v
	}
	return out, nil
}

func (m *mysqlProber) Health() (*HealthInfo, error) {
	st, err := m.kv("SHOW GLOBAL STATUS")
	if err != nil {
		return nil, err
	}
	va, err := m.kv("SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, err
	}
	ver, _ := m.db.Query("SELECT VERSION()")
	var version string
	if ver != nil {
		if ver.Next() {
			_ = ver.Scan(&version)
		}
		ver.Close()
	}
	uptime := atoi(st["Uptime"])
	if uptime <= 0 {
		uptime = 1
	}
	questions := atoi(st["Questions"])
	comCommit := atoi(st["Com_commit"])
	comRollback := atoi(st["Com_rollback"])
	maxConn := atoi(va["max_connections"])
	if maxConn <= 0 {
		maxConn = 151
	}
	threadsConn := atoi(st["Threads_connected"])
	bpReq := atoi(st["Innodb_buffer_pool_read_requests"])
	bpReads := atoi(st["Innodb_buffer_pool_reads"])
	hit := 100.0
	if bpReq > 0 {
		hit = 100 * (1 - float64(bpReads)/float64(bpReq))
	}
	return &HealthInfo{
		Version: version, UptimeS: uptime,
		QPS:                round2(float64(questions) / float64(uptime)),
		TPS:                round2(float64(comCommit+comRollback) / float64(uptime)),
		ThreadsConnected:   threadsConn,
		ThreadsRunning:     atoi(st["Threads_running"]),
		MaxConnections:     maxConn,
		ConnUsagePct:       round2(float64(threadsConn) / float64(maxConn) * 100),
		AbortedConnects:    atoi(st["Aborted_connects"]),
		AbortedClients:     atoi(st["Aborted_clients"]),
		BufferPoolHitPct:   round2(hit),
		SlowQueries:        atoi(st["Slow_queries"]),
		CreatedTmpDisk:     atoi(st["Created_tmp_disk_tables"]),
		SortMergePasses:    atoi(st["Sort_merge_passes"]),
		InnodbRowRead:      atoi(st["Innodb_rows_read"]),
		InnodbRowInserted:  atoi(st["Innodb_rows_inserted"]),
		InnodbRowUpdated:   atoi(st["Innodb_rows_updated"]),
		InnodbRowDeleted:   atoi(st["Innodb_rows_deleted"]),
		BytesReceivedPerS: round2(ftoi(st["Bytes_received"]) / float64(uptime)),
		BytesSentPerS:     round2(ftoi(st["Bytes_sent"]) / float64(uptime)),
		CollectedAt:       time.Now(),
	}, nil
}

func round2(v float64) float64 { return float64(int64(v*100)) / 100 }

func (m *mysqlProber) Replication() (*ReplicationInfo, error) {
	rows, err := m.db.Query("SHOW REPLICA STATUS")
	if err != nil {
		rows, err = m.db.Query("SHOW SLAVE STATUS")
		if err != nil {
			return &ReplicationInfo{Enabled: false}, nil
		}
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	if !rows.Next() {
		return &ReplicationInfo{Enabled: false}, nil
	}
	raw := make([]sql.NullString, len(cols))
	ptr := make([]interface{}, len(cols))
	for i := range raw {
		ptr[i] = &raw[i]
	}
	if err := rows.Scan(ptr...); err != nil {
		return nil, err
	}
	mv := map[string]string{}
	for i, c := range cols {
		mv[c] = raw[i].String
	}
	lag, _ := strconv.ParseInt(mv["Seconds_Behind_Master"], 10, 64)
	return &ReplicationInfo{
		Enabled: true,
		IORunning:          mv["Replica_IO_Running"] + mv["Slave_IO_Running"],
		SQLRunning:         mv["Replica_SQL_Running"] + mv["Slave_SQL_Running"],
		SecondsBehindMaster: lag,
		MasterHost:         mv["Master_Host"] + mv["Source_Host"],
		AutoPosition:       mv["Auto_Position"] == "1",
	}, nil
}

func (m *mysqlProber) Sessions() ([]SessionInfo, error) {
	rows, err := m.db.Query(`SELECT id,user,host,IFNULL(db,''),command,time,IFNULL(state,''),LEFT(IFNULL(info,''),500)
		FROM information_schema.processlist ORDER BY time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionInfo{}
	for rows.Next() {
		var s SessionInfo
		if err := rows.Scan(&s.ID, &s.User, &s.Host, &s.DB, &s.Command, &s.TimeS, &s.State, &s.Info); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (m *mysqlProber) Kill(id int64) error {
	_, err := m.db.Exec(fmt.Sprintf("KILL %d", id))
	return err
}

func (m *mysqlProber) LongTransactions() ([]LongTxn, error) {
	rows, err := m.db.Query(`SELECT trx_id,trx_state,TIMESTAMPDIFF(SECOND,trx_started,NOW()),
		IFNULL(trx_rows_locked,0),IFNULL(trx_rows_modified,0),LEFT(IFNULL(trx_query,''),300)
		FROM information_schema.innodb_trx ORDER BY trx_started LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LongTxn{}
	for rows.Next() {
		var t LongTxn
		if err := rows.Scan(&t.TrxID, &t.State, &t.StartedS, &t.RowsLocked, &t.RowsModified, &t.Query); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (m *mysqlProber) Tables() ([]SchemaTableInfo, error) {
	rows, err := m.db.Query(`SELECT table_schema,table_name,IFNULL(engine,''),IFNULL(table_rows,0),
		IFNULL(data_length,0),IFNULL(index_length,0),IFNULL(data_free,0)
		FROM information_schema.tables
		WHERE table_schema NOT IN ('mysql','information_schema','performance_schema','sys')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pk := m.primaryKeys()
	out := []SchemaTableInfo{}
	for rows.Next() {
		var t SchemaTableInfo
		var data, idx, free int64
		if err := rows.Scan(&t.SchemaName, &t.TableName, &t.Engine, &t.TableRows, &data, &idx, &free); err != nil {
			continue
		}
		t.DataMB = round2(float64(data) / 1024 / 1024)
		t.IndexMB = round2(float64(idx) / 1024 / 1024)
		t.FreeMB = round2(float64(free) / 1024 / 1024)
		t.HasPrimary = pk[t.SchemaName+"."+t.TableName]
		out = append(out, t)
	}
	return out, nil
}

func (m *mysqlProber) primaryKeys() map[string]bool {
	out := map[string]bool{}
	rows, err := m.db.Query(`SELECT table_schema,table_name FROM information_schema.statistics
		WHERE index_name='PRIMARY' GROUP BY table_schema,table_name`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var s, t string
		if rows.Scan(&s, &t) == nil {
			out[s+"."+t] = true
		}
	}
	return out
}

func (m *mysqlProber) SlowTop(n int) ([]SlowDigest, error) {
	if n <= 0 {
		n = 20
	}
	rows, err := m.db.Query(`SELECT IFNULL(schema_name,''),IFNULL(digest_text,''),count_star,
		avg_timer_wait,max_timer_wait,IFNULL(avg_rows_examined,0),IFNULL(avg_rows_sent,0)
		FROM performance_schema.events_statements_summary_by_digest
		ORDER BY sum_timer_wait DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SlowDigest{}
	for rows.Next() {
		var d SlowDigest
		var avgPs, maxPs float64
		if err := rows.Scan(&d.Schema, &d.SampleSQL, &d.ExecCount, &avgPs, &maxPs,
			&d.AvgRowsExamined, &d.AvgRowsSent); err != nil {
			continue
		}
		d.AvgLatencyMS = round2(avgPs / 1e9)
		d.MaxLatencyMS = round2(maxPs / 1e9)
		d.TotalLatencyS = round2(float64(d.ExecCount) * avgPs / 1e12)
		d.FullScan = d.AvgRowsExamined > 10000 && d.AvgRowsSent < d.AvgRowsExamined/100
		out = append(out, d)
	}
	return out, nil
}

func (m *mysqlProber) Query(sqlText string, maxRows int) (*QueryResult, error) {
	return m.run(sqlText, maxRows)
}
func (m *mysqlProber) Explain(sqlText string) (*QueryResult, error) {
	return m.run("EXPLAIN "+sqlText, 200)
}

func (m *mysqlProber) run(sqlText string, maxRows int) (*QueryResult, error) {
	start := time.Now()
	rows, err := m.db.Query(sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	res := &QueryResult{Columns: cols}
	for rows.Next() && (maxRows <= 0 || len(res.Rows) < maxRows) {
		buf := make([]sql.NullString, len(cols))
		ptr := make([]interface{}, len(cols))
		for i := range buf {
			ptr[i] = &buf[i]
		}
		if err := rows.Scan(ptr...); err != nil {
			return nil, err
		}
		line := make([]string, len(cols))
		for i := range buf {
			line[i] = buf[i].String
		}
		res.Rows = append(res.Rows, line)
	}
	res.CostMS = time.Since(start).Milliseconds()
	return res, nil
}
