package com.ousiassllc.swarmviewer.ui.pairing

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ExperimentalGetImage
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.barcode.BarcodeScanner
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import com.ousiassllc.swarmviewer.ui.theme.AppButton
import com.ousiassllc.swarmviewer.ui.theme.LocalGutter
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

private enum class CameraPermission { Checking, Granted, Denied }

/**
 * QR 読み取り画面。**カメラ権限は、この画面を開いた（QR 読み取りを開始した）ときだけ要求する**。
 * 拒否されたら貼り付けへ誘導する（他の経路は権限なしで使える）。読み取った文字列は貼り付けと同じ経路（[onScanned]）に渡す。
 * 文字列（token を含む）はログに出さず、この画面にも表示しない。
 */
@Composable
fun QrScanScreen(
    onScanned: (String) -> Unit,
    onBack: () -> Unit,
) {
    val context = LocalContext.current
    var permission by remember { mutableStateOf(if (hasCameraPermission(context)) CameraPermission.Granted else CameraPermission.Checking) }
    val launcher =
        rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            permission = if (granted) CameraPermission.Granted else CameraPermission.Denied
        }
    LaunchedEffect(Unit) {
        if (permission == CameraPermission.Checking) launcher.launch(Manifest.permission.CAMERA)
    }
    BackHandler(onBack = onBack)

    when (permission) {
        CameraPermission.Granted -> ScannerView(onScanned = onScanned, onBack = onBack)
        CameraPermission.Checking -> Centered { Text("カメラの許可を確認しています…") }
        CameraPermission.Denied ->
            Centered {
                Text("カメラの許可がないため、QR を読み取れません。接続文字列を貼り付けて接続してください。")
                AppButton(onClick = onBack) { Text("貼り付けに戻る") }
            }
    }
}

@Composable
private fun Centered(content: @Composable () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(LocalGutter.current),
        verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) { content() }
}

private fun hasCameraPermission(context: Context): Boolean =
    ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED

@Composable
private fun ScannerView(
    onScanned: (String) -> Unit,
    onBack: () -> Unit,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val currentOnScanned by rememberUpdatedState(onScanned)
    val executor = remember { Executors.newSingleThreadExecutor() }
    val scanner =
        remember {
            BarcodeScanning.getClient(BarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build())
        }
    var provider by remember { mutableStateOf<ProcessCameraProvider?>(null) }
    DisposableEffect(Unit) {
        onDispose {
            provider?.unbindAll()
            scanner.close()
            executor.shutdown()
        }
    }

    Box(modifier = Modifier.fillMaxSize()) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                PreviewView(ctx).also { view ->
                    val future = ProcessCameraProvider.getInstance(ctx)
                    future.addListener({
                        val cameraProvider = future.get()
                        provider = cameraProvider
                        val preview = Preview.Builder().build().also { it.surfaceProvider = view.surfaceProvider }
                        val analysis =
                            ImageAnalysis
                                .Builder()
                                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                                .build()
                                .also { it.setAnalyzer(executor, QrAnalyzer(scanner) { raw -> currentOnScanned(raw) }) }
                        cameraProvider.unbindAll()
                        cameraProvider.bindToLifecycle(lifecycleOwner, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
                    }, ContextCompat.getMainExecutor(ctx))
                }
            },
        )
        Column(
            modifier = Modifier.align(Alignment.BottomCenter).padding(LocalGutter.current),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // カメラ映像の明るさに依らず読めるよう、文言は ink の帯（inverseSurface + inverseOnSurface）に載せる（C5）。
            Surface(
                color = MaterialTheme.colorScheme.inverseSurface,
                contentColor = MaterialTheme.colorScheme.inverseOnSurface,
                shape = MaterialTheme.shapes.medium,
            ) {
                Text("接続文字列の QR コードを枠に映してください", modifier = Modifier.padding(16.dp))
            }
            AppButton(onClick = onBack) { Text("貼り付けに戻る") }
        }
    }
}

/** 最初に読めた QR の文字列を 1 回だけ [onResult]（メインスレッド）へ渡す。読めた内容はログに出さない。 */
private class QrAnalyzer(
    private val scanner: BarcodeScanner,
    private val onResult: (String) -> Unit,
) : ImageAnalysis.Analyzer {
    private val done = AtomicBoolean(false)

    @androidx.annotation.OptIn(ExperimentalGetImage::class)
    override fun analyze(imageProxy: ImageProxy) {
        val media = imageProxy.image
        if (media == null || done.get()) {
            imageProxy.close()
            return
        }
        scanner
            .process(InputImage.fromMediaImage(media, imageProxy.imageInfo.rotationDegrees))
            .addOnSuccessListener { codes ->
                val raw = codes.firstNotNullOfOrNull { it.rawValue }
                if (raw != null && done.compareAndSet(false, true)) onResult(raw)
            }.addOnCompleteListener { imageProxy.close() }
    }
}
