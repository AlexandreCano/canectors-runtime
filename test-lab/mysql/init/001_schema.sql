-- Minimal deterministic schema for the mysql driver scenarios.
CREATE TABLE IF NOT EXISTS mysql_customers (
    external_id VARCHAR(64) NOT NULL PRIMARY KEY,
    email       VARCHAR(255) NOT NULL,
    segment     VARCHAR(64)
);

INSERT INTO mysql_customers (external_id, email, segment) VALUES
    ('MY-001', 'mysql.one@example.test', 'premium'),
    ('MY-002', 'mysql.two@example.test', 'standard'),
    ('MY-003', 'mysql.three@example.test', 'trial');

CREATE TABLE IF NOT EXISTS mysql_dest_customers (
    external_id VARCHAR(64) NOT NULL PRIMARY KEY,
    email       VARCHAR(255) NOT NULL,
    segment     VARCHAR(64)
);
