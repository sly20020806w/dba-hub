package sqlsafe

import "testing"

func TestAllowReadOnly(t *testing.T) {
	cases := map[string]bool{
		"SELECT * FROM users WHERE id=1": true,
		"show global status":             true,
		"EXPLAIN SELECT * FROM t":        true,
		"desc orders":                    true,
	}
	for q, want := range cases {
		if v := Check(q); v.Allowed != want || !v.ReadOnly {
			t.Fatalf("应放行只读语句: %s -> %+v", q, v)
		}
	}
}

func TestBlockDangerous(t *testing.T) {
	bad := []string{
		"DELETE FROM orders",                 // 无 where
		"UPDATE users SET x=1",               // 无 where
		"DROP TABLE orders",                  // ddl
		"TRUNCATE TABLE orders",
		"SELECT * FROM t; DROP TABLE t",      // 多语句夹带
		"SELECT * FROM t INTO OUTFILE '/tmp/x'",
	}
	for _, q := range bad {
		if v := Check(q); v.Allowed {
			t.Fatalf("应拦截却放行: %s -> %+v", q, v)
		}
	}
}

func TestWriteWithWhereStillBlockedInReadOnly(t *testing.T) {
	v := Check("UPDATE users SET name='x' WHERE id=1")
	if v.Allowed || v.ReadOnly {
		t.Fatalf("只读工作台即便带 WHERE 也应禁止写: %+v", v)
	}
}

func TestEnsureLimit(t *testing.T) {
	if got := EnsureLimit("SELECT * FROM t", 200); got != "SELECT * FROM t LIMIT 200" {
		t.Fatalf("自动补 LIMIT 失败: %s", got)
	}
	orig := "SELECT * FROM t LIMIT 10"
	if got := EnsureLimit(orig, 200); got != orig {
		t.Fatalf("已有 LIMIT 不应重复追加: %s", got)
	}
}
