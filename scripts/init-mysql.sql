-- MySQL 5.7 初始化脚本（仅负责创建数据库与字符集）
-- 注意：所有业务表（users / records / login_logs / operation_logs / refresh_tokens）
-- 均由应用启动时的「版本化迁移」自动创建并维护（见 internal/database/database.go），
-- 这里不再重复建表，避免两套 schema 漂移。

CREATE DATABASE IF NOT EXISTS account_service
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
