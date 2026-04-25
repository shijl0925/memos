-- migration_history
CREATE TABLE IF NOT EXISTS migration_history (
  version VARCHAR(255) NOT NULL PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);

-- system_setting
CREATE TABLE IF NOT EXISTS system_setting (
  name VARCHAR(255) NOT NULL,
  value TEXT NOT NULL,
  description TEXT NOT NULL,
  UNIQUE KEY uq_system_setting_name (name)
);

-- user
CREATE TABLE IF NOT EXISTS `user` (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  row_status VARCHAR(32) NOT NULL DEFAULT 'NORMAL',
  username VARCHAR(255) NOT NULL,
  role VARCHAR(32) NOT NULL DEFAULT 'USER',
  email VARCHAR(255) NOT NULL DEFAULT '',
  nickname VARCHAR(255) NOT NULL DEFAULT '',
  password_hash VARCHAR(255) NOT NULL,
  open_id VARCHAR(255) NOT NULL,
  avatar_url VARCHAR(2048) NOT NULL DEFAULT '',
  UNIQUE KEY uq_user_username (username),
  UNIQUE KEY uq_user_open_id (open_id)
);

-- user_setting
CREATE TABLE IF NOT EXISTS user_setting (
  user_id INT NOT NULL,
  `key` VARCHAR(255) NOT NULL,
  value TEXT NOT NULL,
  UNIQUE KEY uq_user_setting (user_id, `key`)
);

-- memo
CREATE TABLE IF NOT EXISTS memo (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id INT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  row_status VARCHAR(32) NOT NULL DEFAULT 'NORMAL',
  content LONGTEXT NOT NULL,
  visibility VARCHAR(32) NOT NULL DEFAULT 'PRIVATE'
);

-- memo_organizer
CREATE TABLE IF NOT EXISTS memo_organizer (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  memo_id INT NOT NULL,
  user_id INT NOT NULL,
  pinned TINYINT(1) NOT NULL DEFAULT 0,
  UNIQUE KEY uq_memo_organizer (memo_id, user_id)
);

-- shortcut
CREATE TABLE IF NOT EXISTS shortcut (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id INT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  row_status VARCHAR(32) NOT NULL DEFAULT 'NORMAL',
  title VARCHAR(255) NOT NULL DEFAULT '',
  payload TEXT NOT NULL
);

-- resource
CREATE TABLE IF NOT EXISTS resource (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id INT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  filename VARCHAR(255) NOT NULL DEFAULT '',
  `blob` LONGBLOB DEFAULT NULL,
  external_link TEXT NOT NULL,
  type VARCHAR(255) NOT NULL DEFAULT '',
  size BIGINT NOT NULL DEFAULT 0,
  internal_path TEXT NOT NULL
);

-- memo_resource
CREATE TABLE IF NOT EXISTS memo_resource (
  memo_id INT NOT NULL,
  resource_id INT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  updated_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  UNIQUE KEY uq_memo_resource (memo_id, resource_id)
);

-- memo_relation
CREATE TABLE IF NOT EXISTS memo_relation (
  memo_id INT NOT NULL,
  related_memo_id INT NOT NULL,
  type VARCHAR(64) NOT NULL,
  UNIQUE KEY uq_memo_relation (memo_id, related_memo_id, type)
);

-- tag
CREATE TABLE IF NOT EXISTS tag (
  name VARCHAR(255) NOT NULL,
  creator_id INT NOT NULL,
  UNIQUE KEY uq_tag (name, creator_id)
);

-- activity
CREATE TABLE IF NOT EXISTS activity (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  creator_id INT NOT NULL,
  created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
  type VARCHAR(255) NOT NULL DEFAULT '',
  level VARCHAR(32) NOT NULL DEFAULT 'INFO',
  payload TEXT NOT NULL
);

-- storage
CREATE TABLE IF NOT EXISTS storage (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  type VARCHAR(255) NOT NULL,
  config TEXT NOT NULL
);

-- idp
CREATE TABLE IF NOT EXISTS idp (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  type VARCHAR(255) NOT NULL,
  identifier_filter VARCHAR(255) NOT NULL DEFAULT '',
  config TEXT NOT NULL
);
