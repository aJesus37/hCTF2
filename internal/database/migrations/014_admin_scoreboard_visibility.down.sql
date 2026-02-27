-- Remove admin visibility setting
DELETE FROM site_settings WHERE key = 'admin_visible_in_scoreboard';
