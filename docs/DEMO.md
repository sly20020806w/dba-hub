# 10 分钟演示脚本（面试 / 阿里云 ECS）

## 路径 A：零依赖最快演示（不装 MySQL，1 分钟）

```bash
CGO_ENABLED=0 go build -o hub ./cmd/hub && ./hub
# 或直接用交叉编译好的 hub-linux-amd64：chmod +x hub-linux-amd64 && ./hub-linux-amd64
```

打开 `http://<ECS公网IP>:8091/ui/`（安全组放行 8091），系统已自带 `demo-order-mysql` 实例，按下面顺序讲。

1. **健康巡检** → 点「立即巡检」：得到 73 分（warn），逐条讲检查项——连接使用率、缓冲池命中率、主从延迟 6s、412s 长事务（crit）、1 张无主键表、1 张高碎片表；强调每个 warn/crit 都带**当前值和处置建议**。
2. **慢查询** → 点采集：按累计耗时排序，指出前两条 `AvgRowsExamined` 几千万但只返回几行 → **全表扫描**，结合 EXPLAIN 讲索引缺失（对应课程 M8「先查索引问题」）。
3. **会话/事务**：展示 processlist 和 innodb_trx，演示对异常会话点 KILL。
4. **库表容量**：展示大表 TOP、数据/索引/碎片空间、无主键表标记。
5. **SQL 工作台**（亮点）：
   - 正常 `SELECT ... WHERE status=1` → 自动补 `LIMIT 200`，返回结果；
   - `DROP TABLE orders` → **被拦**：只读工作台禁止 DDL；
   - `SELECT * FROM t; DELETE FROM t` → **被拦**：禁止多语句夹带；
   - `DELETE FROM orders` → **被拦**：缺少 WHERE；
   - 切「操作审计」看到所有执行/拦截记录。讲清楚：给开发开放查询但不给他闯祸的能力。

## 路径 B：真实 MySQL 全栈（docker compose，5 分钟，更有说服力）

```bash
docker compose up -d --build
docker compose ps   # hub/mysql/exporter/prometheus/grafana 全部 Up
```

1. 平台「新增实例」：`host=mysql port=3306 user=root password=example mode=mysql`；
2. 对这个**真实 MySQL 8** 点巡检/慢查/库表，数据全部来自 information_schema/performance_schema 实时查询；
3. Grafana（3000, admin/admin）打开 MySQL 看板，讲“平台做巡检分析、Prometheus+exporter 做长期趋势”的分工（对应课程 M5 集群外采集数据库）；
4. 想制造慢查询：`docker exec -it dbahub-mysql mysql -uroot -pexample -e "SELECT count(*) FROM order_db.orders a JOIN order_db.orders b;"`

## 面试高频追问应答

- **健康分怎么算？** 满分 100，每个 crit 扣 15、warn 扣 6，下限 0；阈值都是生产经验值（连接使用率 70/85、命中率 99/95、复制延迟 10s/60s、长事务 60s/300s），可在 inspect 包里调整。
- **慢查询从哪来，为什么用 digest？** `performance_schema.events_statements_summary_by_digest` 把 SQL 归一化后聚合，避免日志刷屏；按 sum_timer_wait 排序能找到“累计最耗资源”而非“单次最慢”的 SQL，这才是优化收益最大的。
- **怎么判断全表扫描？** 平均扫描行远大于平均返回行（>1万行且返回<1%），再用 EXPLAIN 的 type=ALL/key=NULL 佐证。
- **工作台怎么保证安全？** 语句先过规则引擎：识别首词类型、拦截多语句/OUTFILE、只读白名单只放 SELECT/SHOW/EXPLAIN 等、SELECT 强制 LIMIT，所有请求落审计表；规则引擎是纯函数并配了单元测试。
- **平台自身会不会成为单点？** 元数据默认 SQLite 单实例，切 MySQL 即可水平多副本；目标库连接按需建立、设超时和连接上限。
- **无主键表为什么是隐患？** 主从复制 row 模式下无主键会全表扫描回放、放大延迟，在线 DDL 也更危险，所以作为独立巡检项。
