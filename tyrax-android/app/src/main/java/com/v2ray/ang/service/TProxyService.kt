package com.v2ray.ang.service

/**
 * JNI binding for the bundled hev-socks5-tunnel engine
 * (jniLibs/<abi>/libhev-socks5-tunnel.so).
 *
 * The native library registers its methods (via RegisterNatives in JNI_OnLoad)
 * against the fully-qualified class name `com/v2ray/ang/service/TProxyService`
 * — that path is baked into the .so, so this package + class name must NOT be
 * renamed or the natives won't bind (UnsatisfiedLinkError at runtime).
 *
 * Verified against the shipped binary's symbol table:
 *   TProxyStartService(String configPath, int fd)
 *   TProxyStopService()
 *   TProxyGetStats(): long[]   // [tx, rx]
 */
object TProxyService {
    init {
        System.loadLibrary("hev-socks5-tunnel")
    }

    external fun TProxyStartService(configPath: String, fd: Int)
    external fun TProxyStopService()
    external fun TProxyGetStats(): LongArray
}
