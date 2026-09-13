package api

import (
	"encoding/json"
	"strconv"
	"time"

	"dba-hub/internal/inspect"
	"dba-hub/internal/model"
	"dba-hub/internal/mysqlx"
	"dba-hub/internal/sqlsafe"

	"github.com/gin-gonic/gin"
)

func (s *Server) listInstances(c *gin.Context) {
	var out []model.DBInstance
	q := s.db.Order("id desc")
	if env := c.Query("env"); env != "" {
		q = q.Where("env = ?", env)
	}
	if kw := c.Query("kw"); kw != "" {
		q = q.Where("name like ? OR business like ? OR service_node like ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
	}
	q.Find(&out)
	c.JSON(200, out)
}

func (s *Server) loadProber(c *gin.Context) (*model.DBInstance, mysqlx.Prober, bool) {
	id, _ := strconv.Atoi(c.Param("id"))
	var ins model.DBInstance
	if err := s.db.First(&ins, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "实例不存在"})
		return nil, nil, false
	}
	p, err := mysqlx.NewProber(ins)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return nil, nil, false
	}
	return &ins, p, true
}

func (s *Server) ping(c *gin.Context) {
	ins, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	status, ver := "ok", ins.Version
	if err := p.Ping(); err != nil {
		status = "down"
		s.db.Model(ins).Updates(map[string]interface{}{"status": status})
		c.JSON(200, gin.H{"status": status, "error": err.Error()})
		return
	}
	if h, err := p.Health(); err == nil && h.Version != "" {
		ver = h.Version
	}
	now := time.Now()
	s.db.Model(ins).Updates(map[string]interface{}{"status": status, "version": ver, "last_check_at": now})
	c.JSON(200, gin.H{"status": status, "version": ver})
}

func (s *Server) health(c *gin.Context) {
	_, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	h, err := p.Health()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, h)
}

func (s *Server) replication(c *gin.Context) {
	_, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	r, err := p.Replication()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, r)
}

func (s *Server) sessions(c *gin.Context) {
	_, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	list, err := p.Sessions()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, list)
}

func (s *Server) killSession(c *gin.Context) {
	_, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	sid, _ := strconv.ParseInt(c.Param("sid"), 10, 64)
	if err := p.Kill(sid); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (s *Server) longTxn(c *gin.Context) {
	_, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	list, err := p.LongTransactions()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, list)
}

func (s *Server) tables(c *gin.Context) {
	ins, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	list, err := p.Tables()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 刷新快照（先删旧再写新）
	s.db.Where("instance_id = ?", ins.ID).Delete(&model.TableStat{})
	for _, t := range list {
		var frag float64
		if t.DataMB+t.IndexMB > 0 {
			frag = float64(int64(t.FreeMB/(t.DataMB+t.IndexMB)*1000)) / 10
		}
		s.db.Create(&model.TableStat{
			InstanceID: ins.ID, SchemaName: t.SchemaName, TableName: t.TableName, Engine: t.Engine,
			TableRows: t.TableRows, DataMB: t.DataMB, IndexMB: t.IndexMB, FragmentPct: frag, HasPrimary: t.HasPrimary,
		})
	}
	c.JSON(200, list)
}

func (s *Server) slow(c *gin.Context) {
	ins, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	n, _ := strconv.Atoi(c.DefaultQuery("n", "20"))
	list, err := p.SlowTop(n)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	s.db.Where("instance_id = ?", ins.ID).Delete(&model.SlowQueryStat{})
	for _, d := range list {
		s.db.Create(&model.SlowQueryStat{
			InstanceID: ins.ID, Digest: hashID(d.SampleSQL), Schema: d.Schema, SampleSQL: d.SampleSQL,
			ExecCount: d.ExecCount, AvgLatencyMS: d.AvgLatencyMS, MaxLatencyMS: d.MaxLatencyMS,
			TotalLatencyS: d.TotalLatencyS, AvgRowsExamined: d.AvgRowsExamined,
			AvgRowsSent: d.AvgRowsSent, FullScan: d.FullScan,
		})
	}
	c.JSON(200, list)
}

func (s *Server) inspect(c *gin.Context) {
	ins, p, ok := s.loadProber(c)
	if !ok {
		return
	}
	defer p.Close()
	rep, err := inspect.Run(ins.ID, p)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	itemsJSON, _ := json.Marshal(rep.Items)
	rec := model.InspectionRecord{
		InstanceID: ins.ID, Score: rep.Score, Status: rep.Status, ItemsJSON: string(itemsJSON), CreatedAt: time.Now(),
	}
	s.db.Create(&rec)
	now := time.Now()
	ver := ""
	if rep.Health != nil {
		ver = rep.Health.Version
	}
	s.db.Model(ins).Updates(map[string]interface{}{
		"last_score": rep.Score, "status": rep.Status, "last_check_at": now, "version": ver,
	})
	c.JSON(200, rep)
}

func (s *Server) listInspections(c *gin.Context) {
	var out []model.InspectionRecord
	q := s.db.Order("id desc").Limit(200)
	if id := c.Query("instanceId"); id != "" {
		q = q.Where("instance_id = ?", id)
	}
	q.Find(&out)
	for i := range out {
		_ = json.Unmarshal([]byte(out[i].ItemsJSON), &out[i].Items)
	}
	c.JSON(200, out)
}

type queryReq struct {
	InstanceID uint   `json:"instanceId"`
	SQL        string `json:"sql"`
	Operator   string `json:"operator"`
}

func (s *Server) query(c *gin.Context) {
	var req queryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	v := sqlsafe.Check(req.SQL)
	logRow := model.QueryLog{InstanceID: req.InstanceID, Operator: req.Operator, SQLText: req.SQL,
		ReadOnly: v.ReadOnly, Allowed: v.Allowed, Reason: v.Reason, CreatedAt: time.Now()}
	defer s.db.Create(&logRow)
	if !v.Allowed {
		c.JSON(403, gin.H{"allowed": false, "verdict": v})
		return
	}
	var ins model.DBInstance
	if err := s.db.First(&ins, req.InstanceID).Error; err != nil {
		c.JSON(404, gin.H{"error": "实例不存在"})
		return
	}
	p, err := mysqlx.NewProber(ins)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer p.Close()
	finalSQL := sqlsafe.EnsureLimit(req.SQL, 200)
	res, err := p.Query(finalSQL, 200)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error(), "finalSql": finalSQL})
		return
	}
	logRow.Rows = len(res.Rows)
	logRow.CostMS = res.CostMS
	c.JSON(200, gin.H{"result": res, "verdict": v, "finalSql": finalSQL})
}

func (s *Server) explain(c *gin.Context) {
	var req queryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var ins model.DBInstance
	if err := s.db.First(&ins, req.InstanceID).Error; err != nil {
		c.JSON(404, gin.H{"error": "实例不存在"})
		return
	}
	p, err := mysqlx.NewProber(ins)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer p.Close()
	res, err := p.Explain(req.SQL)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

func (s *Server) listQueryLogs(c *gin.Context) {
	var out []model.QueryLog
	s.db.Order("id desc").Limit(200).Find(&out)
	c.JSON(200, out)
}

func (s *Server) stats(c *gin.Context) {
	var total int64
	s.db.Model(&model.DBInstance{}).Count(&total)
	statusCnt := []map[string]interface{}{}
	s.db.Model(&model.DBInstance{}).Select("status as name, count(*) as count").Group("status").Scan(&statusCnt)
	var avgScore float64
	s.db.Model(&model.DBInstance{}).Where("last_score > 0").Select("COALESCE(avg(last_score),0)").Scan(&avgScore)
	var latest []model.SlowQueryStat
	s.db.Raw(`SELECT t.* FROM slow_query_stats t INNER JOIN (
		SELECT digest, MAX(id) mid FROM slow_query_stats GROUP BY digest) x ON t.id=x.mid
		ORDER BY t.total_latency_s DESC LIMIT 10`).Scan(&latest)
	var inspections int64
	s.db.Model(&model.InspectionRecord{}).Count(&inspections)
	c.JSON(200, gin.H{
		"totalInstances": total, "byStatus": statusCnt, "avgScore": float64(int64(avgScore*10)) / 10,
		"topSlow": latest, "inspectionCount": inspections,
	})
}
