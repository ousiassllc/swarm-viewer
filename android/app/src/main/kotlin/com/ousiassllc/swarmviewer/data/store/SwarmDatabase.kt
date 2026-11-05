package com.ousiassllc.swarmviewer.data.store

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase

/** DB ファイル名。アプリ専用の内部ストレージ（`Context.getDatabasePath`）に作られ、バックアップ・端末間転送の対象外。 */
const val DATABASE_NAME = "viewer.db"

/**
 * 接続・設定・スナップショット・通知済み状態の Room データベース（ER §7）。
 * スキーマは `android/app/schemas/` に書き出す。最初の版（1）でマイグレーションは持たない。
 * 破壊的な再作成（`fallbackToDestructiveMigration`）は使わない（NFR-CMP-4）。外部キーは Room が有効にして開く。
 */
@Database(
    entities = [ConnectionEntity::class, SettingsEntity::class, SnapshotEntity::class, NotifiedStateEntity::class],
    version = 1,
    exportSchema = true,
)
abstract class SwarmDatabase : RoomDatabase() {
    abstract fun connections(): ConnectionDao

    abstract fun settings(): SettingsDao

    abstract fun snapshots(): SnapshotDao

    abstract fun notifiedStates(): NotifiedStateDao

    companion object {
        fun open(context: Context): SwarmDatabase =
            Room.databaseBuilder(context.applicationContext, SwarmDatabase::class.java, DATABASE_NAME).build()
    }
}
