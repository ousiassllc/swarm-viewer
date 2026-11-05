package com.ousiassllc.swarmviewer.data

import android.database.sqlite.SQLiteConstraintException
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.ousiassllc.swarmviewer.domain.Connection
import com.ousiassllc.swarmviewer.domain.Settings
import com.ousiassllc.swarmviewer.domain.SettingsException
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/** 接続・設定の保存（実際の SQLite で確かめる）。 */
@RunWith(AndroidJUnit4::class)
internal class RoomConnectionStoreTest : RoomStoreTestBase() {
    @Test
    fun savedConnectionsComeBackInCreationOrder() =
        runBlocking {
            store.saveConnection(connection("b", URL_2).copy(createdAt = now.plusSeconds(10)))
            store.saveConnection(connection("a", URL_1).copy(createdAt = now))

            assertEquals(listOf("a", "b"), store.connections().map { it.id })
            assertEquals("dev-a", store.connection("a")?.name)
            assertEquals("b", store.connectionByUrl(URL_2)?.id)
            assertNull(store.connection("nope"))
        }

    @Test
    fun savingTheSameUrlKeepsTheIdAndOnlyReplacesTheName() =
        runBlocking {
            store.saveConnection(connection("a", URL_1).copy(createdAt = now))
            store.setLastSession("a", "2")
            store.markBaselined("a", now.plusSeconds(5))

            store.saveConnection(Connection(id = "other-id", name = "renamed", url = URL_1, updatedAt = now.plusSeconds(60)))

            val saved = store.connections().single()
            assertEquals("a", saved.id)
            assertEquals("renamed", saved.name)
            assertEquals("2", saved.lastSession)
            assertEquals(now.plusSeconds(5), saved.baselinedAt)
            assertEquals(now, saved.createdAt)
            assertEquals(now.plusSeconds(60), saved.updatedAt)
        }

    @Test
    fun markBaselinedKeepsTheFirstTimeAndLastSessionCanBeCleared() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            assertTrue(store.markBaselined("a", now))
            assertTrue(store.markBaselined("a", now.plusSeconds(99)))
            assertEquals(now, store.connection("a")?.baselinedAt)

            assertTrue(store.setLastSession("a", "1"))
            assertTrue(store.setLastSession("a", null))
            assertNull(store.connection("a")?.lastSession)
            assertFalse(store.setLastSession("nope", "1"))
            assertFalse(store.markBaselined("nope", now))
        }

    @Test
    fun theSameUrlCannotBeInsertedTwiceDirectly() {
        runBlocking { store.saveConnection(connection("a", URL_1)) }

        assertThrows(SQLiteConstraintException::class.java) {
            db.openHelper.writableDatabase.execSQL(
                "INSERT INTO connections (id, name, url, created_at, updated_at) VALUES ('x', 'n', '$URL_1', 't', 't')",
            )
        }
    }

    @Test
    fun settingsDefaultToTheSpecValuesAndAreValidated() =
        runBlocking {
            assertEquals(Settings(), store.settings())
            assertEquals(15, store.settings().notifyPollMinutes)

            store.saveConnection(connection("a", URL_1))
            val changed =
                Settings(
                    notificationsEnabled = false,
                    screenPollSeconds = 30,
                    notifyPollMinutes = 60,
                    debugLog = true,
                    lastConnectionId = "a",
                )
            store.saveSettings(changed)
            assertEquals(changed, store.settings())
            assertEquals(1, count("settings")) // 1 行固定

            assertThrows(SettingsException::class.java) { runBlocking { store.saveSettings(changed.copy(notifyPollMinutes = 5)) } }
            assertEquals(changed, store.settings())
        }

    @Test
    fun deletingTheLastConnectionResetsTheSettingsReference() =
        runBlocking {
            store.saveConnection(connection("a", URL_1))
            store.setLastConnection("a")
            assertEquals("a", store.settings().lastConnectionId)

            store.deleteConnection("a")

            assertNull(store.settings().lastConnectionId)
        }
}
