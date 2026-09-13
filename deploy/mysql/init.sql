-- 演示库初始化：构造贴近电商的库表结构与数据，供 dba-hub 巡检/慢查/容量分析
CREATE DATABASE IF NOT EXISTS order_db CHARACTER SET utf8mb4;
CREATE DATABASE IF NOT EXISTS user_db CHARACTER SET utf8mb4;

USE order_db;

CREATE TABLE IF NOT EXISTS orders (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  order_no VARCHAR(32) NOT NULL,
  status TINYINT NOT NULL DEFAULT 1,
  amount DECIMAL(12,2) NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_status_ctime (status, create_time),
  KEY idx_order_no (order_no)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_items (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  order_id BIGINT NOT NULL,
  sku VARCHAR(32) NOT NULL,
  qty INT NOT NULL DEFAULT 1,
  KEY idx_order_id (order_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS payments (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  pay_no VARCHAR(32) NOT NULL,
  state TINYINT NOT NULL DEFAULT 0,
  KEY idx_pay_no (pay_no)
) ENGINE=InnoDB;

-- 刻意保留一张无主键表，用于演示“无主键表”巡检项
CREATE TABLE IF NOT EXISTS tmp_import_log (
  batch VARCHAR(40),
  line INT,
  msg VARCHAR(255)
) ENGINE=InnoDB;

USE user_db;
CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone VARCHAR(20) NOT NULL,
  nickname VARCHAR(64),
  KEY idx_phone (phone)
) ENGINE=InnoDB;

-- 灌一点样例数据
USE order_db;
INSERT INTO orders(order_no,status,amount) VALUES
 ('NO20260913001',1,99.00),('NO20260913002',2,199.50),('NO20260913003',1,59.90);
INSERT INTO order_items(order_id,sku,qty) VALUES (1,'SKU-A',1),(1,'SKU-B',2),(2,'SKU-C',1);
INSERT INTO payments(pay_no,state) VALUES ('PAY001',1),('PAY002',2);
INSERT INTO tmp_import_log VALUES ('b1',1,'ok'),('b1',2,'ok'),('b2',1,'retry');
USE user_db;
INSERT INTO users(phone,nickname) VALUES ('13800000001','alice'),('13800000002','bob');

-- mysqld-exporter 监控账号（课程 M5-6.4 集群外采集数据库所需最小权限）
CREATE USER IF NOT EXISTS 'exporter'@'%' IDENTIFIED BY 'exporter123';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';
FLUSH PRIVILEGES;
