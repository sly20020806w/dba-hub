package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"dba-hub/internal/api"
	"dba-hub/internal/config"
	"dba-hub/internal/inspect"
	"dba-hub/internal/model"
	"dba-hub/internal/mysqlx"
	"dba-hub/internal/store"

	"gorm.io/gorm"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := store.Open(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}
	seedDemo(db)

	if cfg.Inspect.IntervalS > 0 {
		go autoInspect(db, cfg.Inspect.IntervalS)
	}

	srv := api.New(db, cfg)
	log.Printf("dba-hub 监听 %s ，控制台 http://localhost%s/ui/", cfg.HTTPListen, cfg.HTTPListen)
	if err := http.ListenAndServe(cfg.HTTPListen, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

// seedDemo 首次启动内置一个 demo 实例，保证打开控制台即有数据可看
func seedDemo(db *gorm.DB) {
	var n int64
	db.Model(&model.DBInstance{}).Count(&n)
	if n > 0 {
		return
	}
	db.Create(&model.DBInstance{
		Name: "demo-order-mysql", Env: "demo", Host: "127.0.0.1", Port: 3306,
		Username: "root", Mode: "demo", ServiceNode: "电商/订单中心", Business: "订单交易",
		Tags: model.StringSlice{"demo", "mysql8.0"}, Status: "unknown", Remark: "内置演示实例（不连真实库）",
	})
}

func autoInspect(db *gorm.DB, intervalS int) {
	tk := time.NewTicker(time.Duration(intervalS) * time.Second)
	defer tk.Stop()
	for range tk.C {
		var list []model.DBInstance
		db.Find(&list)
		for _, ins := range list {
			p, err := mysqlx.NewProber(ins)
			if err != nil {
				continue
			}
			rep, err := inspect.Run(ins.ID, p)
			p.Close()
			if err != nil {
				continue
			}
			db.Model(&ins).Updates(map[string]interface{}{"last_score": rep.Score, "status": rep.Status, "last_check_at": time.Now()})
		}
	}
}
