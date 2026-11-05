package com.ousiassllc.swarmviewer.domain

import kotlinx.serialization.json.Json
import java.io.File

/**
 * testdata/vectors 直下の JSON（デスクトップ版の Go テストと共有するテストベクタ。形式は testdata/vectors/README.md）を読む。
 * Gradle のユニットテストの作業ディレクトリは android/app のため、親をたどってリポジトリ直下の testdata/vectors を探す。
 * 未知のキーは読み込みエラーにする（キーの綴り違いでケースが消えないように）。
 */
internal object Vectors {
    val json = Json { ignoreUnknownKeys = false }

    private val dir: File by lazy {
        generateSequence(File("").absoluteFile) { it.parentFile }
            .map { File(it, "testdata/vectors") }
            .firstOrNull { it.isDirectory }
            ?: error("testdata/vectors が見つからない（作業ディレクトリ: ${File("").absoluteFile}）")
    }

    inline fun <reified T> load(name: String): T = json.decodeFromString(read(name))

    fun read(name: String): String = File(dir, name).readText(Charsets.UTF_8)

    /** ケース数の下限。ファイルが空・キーの綴り違いでケースが消えても、テストが静かに空振りしない。 */
    fun requireMin(
        what: String,
        actual: Int,
        min: Int,
    ) {
        check(actual >= min) { "$what のケースが $actual 件しかない（下限 $min）" }
    }
}
