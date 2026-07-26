#!/usr/bin/env python3
"""Create a deterministic SQLite database for test-lab scenarios.

Usage: seed-sqlite.py <db-path>
The file is recreated from scratch on every call so runs stay deterministic.
"""
import os
import sqlite3
import sys

path = sys.argv[1] if len(sys.argv) > 1 else "test-lab/tmp/lab.db"
os.makedirs(os.path.dirname(path), exist_ok=True)
if os.path.exists(path):
    os.remove(path)
conn = sqlite3.connect(path)
conn.executescript(
    """
    CREATE TABLE lab_customers (
        external_id TEXT PRIMARY KEY,
        email       TEXT NOT NULL,
        segment     TEXT
    );
    INSERT INTO lab_customers (external_id, email, segment) VALUES
        ('SQL-001', 'sqlite.one@example.test', 'premium'),
        ('SQL-002', 'sqlite.two@example.test', 'standard'),
        ('SQL-003', 'sqlite.three@example.test', 'trial');
    """
)
conn.commit()
conn.close()
print(f"seeded {path}")
