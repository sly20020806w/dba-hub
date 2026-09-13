package inspect

import (
	"testing"

	"dba-hub/internal/model"
	"dba-hub/internal/mysqlx"
)

func TestRunOnDemo(t *testing.T) {
	ins := model.DBInstance{ID: 1, Mode: "demo"}
	p, err := mysqlx.NewProber(ins)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Run(1, p)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Score < 0 || rep.Score > 100 {
		t.Fatalf("健康分越界: %d", rep.Score)
	}
	if len(rep.Items) < 8 {
		t.Fatalf("巡检项过少: %d", len(rep.Items))
	}
	// demo 数据刻意制造了 6s 复制延迟、1 张无主键表、1 张高碎片表、412s 长事务
	var sawNoPK, sawLongTxn bool
	for _, it := range rep.Items {
		if it.Key == "no_pk" && it.Level == "warn" {
			sawNoPK = true
		}
		if it.Key == "long_txn" && it.Level == "crit" {
			sawLongTxn = true
		}
	}
	if !sawNoPK {
		t.Fatalf("应检出无主键表: %+v", rep.Items)
	}
	if !sawLongTxn {
		t.Fatalf("应检出长事务: %+v", rep.Items)
	}
	if len(rep.NoPKTables) != 1 {
		t.Fatalf("demo 应有 1 张无主键表, 实际 %d", len(rep.NoPKTables))
	}
	if len(rep.BigTables) == 0 || rep.BigTables[0].TableName != "order_items" {
		t.Fatalf("最大表应为 order_items: %+v", rep.BigTables)
	}
}
