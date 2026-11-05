plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.ksp)
    alias(libs.plugins.ktlint)
}

// 版はタグが唯一の元。リリースでは .github/scripts/android-version.sh が決めた値を
// -PappVersionName=X.Y.Z -PappVersionCode=N で渡す。渡さない通常のビルド（CI の assembleDebug など）は次の既定値を使う。
// release ビルドには signingConfig を置かない（署名鍵をリポジトリ・Gradle に持ち込まない）。署名は release.yml の build-android が
// apksigner で行う（docs/environment/setup.md §11.3・§11.4）。
val appVersionName = providers.gradleProperty("appVersionName").getOrElse("0.1.0")
val appVersionCode = providers.gradleProperty("appVersionCode").map(String::toInt).getOrElse(1)

android {
    namespace = "com.ousiassllc.swarmviewer"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.ousiassllc.swarmviewer"
        minSdk = 29
        targetSdk = 35
        versionCode = appVersionCode
        versionName = appVersionName
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    lint {
        warningsAsErrors = true
        abortOnError = true
        // 次の 3 件は「新しい版がある」という指摘で、時間が経つと新しいコミットなしに CI が失敗するため無効にする。
        // compileSdk / targetSdk は親 Issue #7 の確定事項（35）、依存の上限は gradle/libs.versions.toml の先頭コメントを参照。
        disable += setOf("OldTargetApi", "GradleDependency", "NewerVersionAvailable")
    }
}

kotlin {
    jvmToolchain(17)
}

// Room のスキーマ（JSON）を android/app/schemas/ に書き出す。版ごとに差分を追えるようにコミットする。
ksp {
    arg("room.schemaLocation", "$projectDir/schemas")
}

// Go と共有するテストベクタ（testdata/vectors）をテストの入力にする。宣言しないと、ベクタだけを変えたときに
// Gradle が「入力に変更なし」としてテストを再実行せず（ビルドキャッシュも含む）、変更が静かに検証されない。
tasks.withType<Test>().configureEach {
    inputs
        .dir(rootProject.layout.projectDirectory.dir("../testdata/vectors"))
        .withPropertyName("testVectors")
        .withPathSensitivity(PathSensitivity.RELATIVE)
}

dependencies {
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material3.windowsizeclass)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)

    implementation(libs.androidx.camera.core)
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.camera.view)
    implementation(libs.mlkit.barcode.scanning)

    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.okhttp)
    implementation(libs.androidx.room.runtime)
    implementation(libs.androidx.room.ktx)
    ksp(libs.androidx.room.compiler)
    implementation(libs.androidx.work.runtime.ktx)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.turbine)

    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.kotlinx.coroutines.test)
}
