CREATE TABLE IF NOT EXISTS site_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO site_settings (key, value) VALUES
    ('custom_head_html', ''),
    ('custom_body_end_html', ''),
    ('custom_code_pages', '["all"]');
