# ─────────────────────────────────────────────────────────────────────────────
# MahaSwarna ProGuard / R8 rules
# Stack: OkHttp 5, Retrofit 3 + kotlinx.serialization (built-in converter),
#        Hilt 2.52, Room 2.8.3, Firebase BOM 34, Sentry 7.18,
#        Play Billing 7.1.1, Play Integrity 1.4.0, Coil 3, CameraX 1.4.1,
#        security-crypto 1.1.0-alpha06, Vico 2.0.0-beta.3, Paging 3.3.5
# ─────────────────────────────────────────────────────────────────────────────

# ── OkHttp 5 ─────────────────────────────────────────────────────────────────
-dontwarn okhttp3.**
-dontwarn okio.**

# ── Retrofit 3 ───────────────────────────────────────────────────────────────
-keep class retrofit2.** { *; }
# Retrofit 3 uses reflection to find suspend functions on service interfaces.
-keepclassmembers,allowshrinking,allowobfuscation interface * {
    @retrofit2.http.* <methods>;
}

# ── kotlinx.serialization ─────────────────────────────────────────────────────
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keepclassmembers class kotlinx.serialization.json.** { *** Companion; }

# FIX: @Serializable data classes used as Retrofit request/response bodies are
# accessed via generated serializers. R8 strips the companion object and
# serializer() method without these rules, causing silent JSON parse failures
# or NoSuchMethodException at runtime for every API call.
-keepclassmembers @kotlinx.serialization.Serializable class ** {
    *** Companion;
    kotlinx.serialization.KSerializer serializer(...);
}
-keep @kotlinx.serialization.Serializable class * { *; }
# Keep the generated $serializer inner classes produced by the Kotlin compiler
-keepclassmembers class **$$serializer { *; }

# ── Firebase ──────────────────────────────────────────────────────────────────
-keep class com.google.firebase.** { *; }
# Crashlytics needs line numbers for useful crash reports
-keepattributes SourceFile, LineNumberTable
-keep public class * extends java.lang.Exception

# ── Hilt / Dagger ─────────────────────────────────────────────────────────────
-keep class dagger.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.ActivityComponentManager { *; }
-keepnames @dagger.hilt.android.lifecycle.HiltViewModel class *

# ── Room ──────────────────────────────────────────────────────────────────────
-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *
-keep @androidx.room.Dao class *
-keep class * extends androidx.room.paging.LimitOffsetPagingSource { *; }

# ── Sentry ────────────────────────────────────────────────────────────────────
-keep class io.sentry.** { *; }
-dontwarn io.sentry.**

# ── Play Billing 7.1.1 ────────────────────────────────────────────────────────
# FIX: billing-ktx suspend extensions use reflection for continuation interception
# on older API levels. Keep all BillingClient classes to prevent runtime failures.
-keep class com.android.billingclient.** { *; }
-dontwarn com.android.billingclient.**

# ── Play Integrity 1.4.0 ──────────────────────────────────────────────────────
# FIX: IntegrityManagerFactory, IntegrityTokenRequest, IntegrityTokenResponse are
# accessed via Play Services dynamic dispatch. Without keeps, R8 strips
# IntegrityTokenResponse.token() or obfuscates builder methods, causing
# NoSuchMethodException at runtime — breaking device attestation for all users.
-keep class com.google.android.play.core.integrity.** { *; }
-keep interface com.google.android.play.core.integrity.** { *; }
-dontwarn com.google.android.play.core.integrity.**

# ── Coil 3.0.4 ────────────────────────────────────────────────────────────────
# Coil 3 ships consumer rules in its AAR. Suppress residual warnings.
-dontwarn coil3.**

# ── CameraX 1.4.1 ─────────────────────────────────────────────────────────────
# CameraX ships consumer ProGuard rules. Suppress residual warnings.
-dontwarn androidx.camera.**

# ── EncryptedSharedPreferences (security-crypto 1.1.0-alpha06) ───────────────
# Tink internals can fail after obfuscation on API 24–25 via KeyInfo reflection.
-keep class com.google.crypto.tink.** { *; }
-dontwarn com.google.crypto.tink.**

# ── Vico 2.0.0-beta.3 ─────────────────────────────────────────────────────────
# Vico beta ships consumer ProGuard rules in the AAR.
-dontwarn com.patrykandpatrick.vico.**

# ── Kotlin coroutines ─────────────────────────────────────────────────────────
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}
-keepclassmembernames class kotlinx.** { volatile <fields>; }

# ── Kotlin Reflect (Retrofit 3 suspend function detection) ───────────────────
-keep class kotlin.Metadata { *; }
-dontwarn kotlin.reflect.jvm.internal.**
