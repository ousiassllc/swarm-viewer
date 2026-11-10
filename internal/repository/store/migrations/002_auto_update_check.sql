-- 版 2: 新しい版の自動確認の設定（Issue #109）。既定はオン。既存の行にも既定値が入る。
ALTER TABLE settings ADD COLUMN auto_update_check INTEGER NOT NULL DEFAULT 1 CHECK (auto_update_check IN (0, 1));
