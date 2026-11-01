import java.net.URI

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val hevAar = layout.projectDirectory.file("libs/hev-socks5-tunnel.aar")
val coreAar = layout.projectDirectory.file("libs/tunnelcore.aar")

val downloadHevAar by tasks.registering {
    outputs.file(hevAar)
    doLast {
        val output = hevAar.asFile
        output.parentFile.mkdirs()
        URI("https://github.com/heiher/hev-socks5-tunnel/releases/download/2.19.3/hev-socks5-tunnel.aar")
            .toURL().openStream().use { input ->
                output.outputStream().use { target -> input.copyTo(target) }
            }
    }
}

tasks.named("preBuild").configure {
    dependsOn(downloadHevAar)
    dependsOn(tasks.register("verifyCoreAar") {
        doLast {
            check(coreAar.asFile.isFile) {
                "Missing app/libs/tunnelcore.aar. Run scripts/build-android.sh first."
            }
        }
    })
}

android {
    namespace = "dev.kyouji.tunnelgo"
    compileSdk = 35

    defaultConfig {
        applicationId = "dev.kyouji.tunnelgo"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "1.0.0"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    packaging {
        jniLibs {
            useLegacyPackaging = false
        }
    }
}

dependencies {
    implementation(files(hevAar, coreAar))
}
