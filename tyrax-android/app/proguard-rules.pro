# ─────────────────────────────────────────────────────────────────────────────
# TYRAX — ProGuard / R8 rules
# Keep everything required for reflection-based libraries (Gson, Retrofit,
# WireGuard, Hilt) to survive minification in release builds.
# ─────────────────────────────────────────────────────────────────────────────

# ── Attributes ───────────────────────────────────────────────────────────────
# Keep annotations, generic signatures and exceptions — required by Retrofit,
# Gson type adapters and Hilt code generation.
-keepattributes Signature
-keepattributes *Annotation*
-keepattributes RuntimeVisibleAnnotations
-keepattributes RuntimeVisibleParameterAnnotations
-keepattributes AnnotationDefault
-keepattributes InnerClasses
-keepattributes EnclosingMethod
-keepattributes Exceptions
# Preserve line numbers for readable crash traces, then hide source file name.
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile

# ── WireGuard (config parsing + GoBackend / JNI data plane) ──────────────────
-keep class com.wireguard.** { *; }
-keep interface com.wireguard.** { *; }
-keepclassmembers class com.wireguard.** { *; }
-dontwarn com.wireguard.**

# ── Retrofit / OkHttp ─────────────────────────────────────────────────────────
# Keep the API service interface and its method/parameter annotations.
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keepclasseswithmembers class * {
    @retrofit2.http.* <methods>;
}
-keep interface com.tyrax.data.remote.TyraxApiService { *; }
-dontwarn retrofit2.**
-dontwarn okhttp3.**
-dontwarn okio.**
# Retrofit suspend functions use Kotlin Continuation — keep generic signatures.
-keep,allowobfuscation,allowshrinking class kotlin.coroutines.Continuation

# ── Gson — model / DTO data classes (serialized via reflection) ───────────────
-keep class com.tyrax.data.remote.** { *; }
-keep class com.tyrax.domain.model.** { *; }
# Keep @SerializedName field names intact.
-keepclassmembers class com.tyrax.data.remote.** {
    @com.google.gson.annotations.SerializedName <fields>;
}
-keep class com.google.gson.** { *; }
-dontwarn com.google.gson.**
# Keep generic type tokens used by Gson.
-keep class * extends com.google.gson.reflect.TypeToken
-keep,allowobfuscation class com.google.gson.reflect.TypeToken

# ── Hilt / Dagger (generated components) ──────────────────────────────────────
-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.* { *; }
-keep @dagger.hilt.android.lifecycle.HiltViewModel class * { *; }
-keepclasseswithmembers class * {
    @javax.inject.Inject <init>(...);
}
-dontwarn dagger.hilt.**

# ── Kotlin metadata / coroutines ─────────────────────────────────────────────
-keep class kotlin.Metadata { *; }
-dontwarn kotlinx.coroutines.**

# ── Compose ──────────────────────────────────────────────────────────────────
# R8 ships Compose rules, but keep ViewModels referenced by hiltViewModel().
-keep class * extends androidx.lifecycle.ViewModel { *; }
