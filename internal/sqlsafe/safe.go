package sqlsafe

import (
	"regexp"
	"strings"
)

// Verdict SQL 安全判定结果
type Verdict struct {
	StmtType string `json:"stmtType"` // select/show/explain/ddl/dml/...
	ReadOnly bool   `json:"readOnly"`
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason"`
}

var (
	reMulti       = regexp.MustCompile(`;\s*\S`)
	reOutfile     = regexp.MustCompile(`(?i)\b(into\s+(out|dump)file|load_file\s*\()`)
	reFirstWord   = regexp.MustCompile(`^[\s(/*-]*([a-z]+)`)
	reHasLimit    = regexp.MustCompile(`(?i)\blimit\s+\d+`)
	reHasWhere    = regexp.MustCompile(`(?i)\bwhere\b`)
	reWriteNoCond = regexp.MustCompile(`(?i)^(update|delete)\b`)
)

// readOnly 语句前缀集合
var readOnlyStmts = map[string]bool{
	"select": true, "show": true, "explain": true, "desc": true,
	"describe": true, "with": true, "use": true,
}
var ddlStmts = map[string]bool{
	"create": true, "alter": true, "drop": true, "truncate": true,
	"rename": true, "index": true,
}
var writeStmts = map[string]bool{
	"insert": true, "update": true, "delete": true, "replace": true,
	"merge": true, "grant": true, "revoke": true, "set": true, "call": true,
}

// Check 在“只读工作台”语境下判定一条 SQL 是否允许执行
func Check(sqlText string) Verdict {
	s := strings.TrimSpace(sqlText)
	v := Verdict{}

	if s == "" {
		return Verdict{Allowed: false, Reason: "空语句"}
	}
	// 1) 多语句拦截，防止 ; 夹带
	if reMulti.MatchString(strings.TrimSuffix(s, ";")) {
		return Verdict{Allowed: false, Reason: "禁止一次执行多条语句"}
	}
	// 2) 落库/读文件等危险关键字
	if reOutfile.MatchString(s) {
		return Verdict{Allowed: false, Reason: "禁止 INTO OUTFILE / LOAD_FILE 等文件操作"}
	}
	m := reFirstWord.FindStringSubmatch(strings.ToLower(s))
	if len(m) < 2 {
		return Verdict{Allowed: false, Reason: "无法识别语句类型"}
	}
	first := m[1]
	v.StmtType = first

	switch {
	case readOnlyStmts[first]:
		v.ReadOnly = true
		v.Allowed = true
		if first == "select" && !reHasLimit.MatchString(s) {
			v.Reason = "查询未带 LIMIT，平台将自动补 LIMIT 兜底"
		}
		return v
	case ddlStmts[first]:
		v.Reason = "只读工作台禁止 DDL（" + strings.ToUpper(first) + "），变更请走工单/审批"
		return v
	case writeStmts[first]:
		v.ReadOnly = false
		if reWriteNoCond.MatchString(s) && !reHasWhere.MatchString(s) {
			v.Reason = strings.ToUpper(first) + " 缺少 WHERE 条件，已拦截"
			return v
		}
		v.Reason = "只读工作台禁止写操作（" + strings.ToUpper(first) + "）"
		return v
	default:
		v.Reason = "不被允许的语句类型: " + strings.ToUpper(first)
		return v
	}
}

// EnsureLimit 为没有 LIMIT 的 SELECT 追加兜底 limit
func EnsureLimit(sqlText string, maxRows int) string {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sqlText), ";"))
	v := Check(s)
	if v.StmtType == "select" && !reHasLimit.MatchString(s) {
		return s + " LIMIT " + itoa(maxRows)
	}
	return s
}

func itoa(n int) string {
	if n <= 0 {
		n = 100
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if len(b) == 0 {
		b = []byte{'0'}
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
