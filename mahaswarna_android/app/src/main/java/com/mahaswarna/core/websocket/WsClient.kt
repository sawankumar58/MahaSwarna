package com.mahaswarna.core.websocket

import com.google.firebase.crashlytics.FirebaseCrashlytics
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Singleton
import javax.net.ssl.SSLHandshakeException

/**
 * OkHttp 5 WebSocket wrapper.
 *
 * connect(token): opens the socket. Requires a confirmed valid JWT.
 *   JWT pre-warm in MainActivity at T+80ms — see MainActivity for details.
 * disconnect(): graceful close (code 1000).
 * messageFlow(): cold Flow of [WsEnvelope] — uses callbackFlow + awaitClose
 *   to avoid leaking the socket on collector cancellation.
 * connectionState: StateFlow<WsConnectionState> — observed by ViewModels
 *   to drive StaleRateBanner and polling fallback.
 *
 * Backoff: 1s → 2s → 4s → … cap 60s; resets to 0 on Connected.
 * Retryable errors: IOException, generic Throwable (not SSLHandshakeException).
 * Non-retryable: SSLHandshakeException (cert mismatch) → transitions to Error.
 */
@Singleton
class WsClient @Inject constructor(
    @Named("ws") private val okHttpClient: OkHttpClient,
    private val json: Json,
) {
    private val _connectionState = MutableStateFlow<WsConnectionState>(WsConnectionState.Disconnected)
    val connectionState: StateFlow<WsConnectionState> = _connectionState.asStateFlow()

    @Volatile private var socket: WebSocket? = null
    @Volatile private var backoffMs: Long = 1_000L
    @Volatile private var active: Boolean = false

    /**
     * Connects to the WS endpoint with the given JWT.
     * Implements exponential backoff (1s … 60s) for transient failures.
     * Non-retryable SSLHandshakeException → Error state.
     */
    suspend fun connect(token: String, wsUrl: String) {
        active = true
        backoffMs = 1_000L
        _connectionState.value = WsConnectionState.Connecting

        while (active) {
            val connected = tryConnect(token, wsUrl)
            if (!active) break
            if (!connected) {
                _connectionState.value = WsConnectionState.Reconnecting
                delay(backoffMs)
                backoffMs = (backoffMs * 2).coerceAtMost(60_000L)
            }
        }
    }

    /** @return true if the connection was established and closed normally; false on failure. */
    private suspend fun tryConnect(token: String, wsUrl: String): Boolean {
        var succeeded = false
        val request = Request.Builder()
            .url(wsUrl)
            .header("Authorization", "Bearer $token")
            .build()

        val latch = java.util.concurrent.CountDownLatch(1)
        val listener = object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                socket = webSocket
                _connectionState.value = WsConnectionState.Connected
                backoffMs = 1_000L   // reset on successful connect
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                // Messages flow through messageFlow() via shared callbackFlow
                _messageListeners.forEach { it(text) }
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                webSocket.close(1000, null)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                succeeded = true
                socket = null
                _connectionState.value = WsConnectionState.Disconnected
                latch.countDown()
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                socket = null
                if (t is SSLHandshakeException) {
                    // Non-retryable — cert mismatch; transition to Error immediately
                    _connectionState.value = WsConnectionState.Error(t)
                    active = false
                    FirebaseCrashlytics.getInstance().recordException(t)
                } else {
                    // Retryable (IOException, timeout, etc.)
                    _connectionState.value = WsConnectionState.Reconnecting
                }
                latch.countDown()
            }
        }

        okHttpClient.newWebSocket(request, listener)
        // Block until the socket closes or fails (coroutine-friendly via runInterruptible)
        kotlinx.coroutines.runInterruptible { latch.await() }
        return succeeded
    }

    fun disconnect() {
        active = false
        socket?.close(1000, "client disconnect")
        socket = null
        _connectionState.value = WsConnectionState.Disconnected
    }

    // ── Message Flow ──────────────────────────────────────────────────────────

    private val _messageListeners = mutableListOf<(String) -> Unit>()

    /**
     * Cold flow of parsed [WsEnvelope]s.
     * Uses callbackFlow + awaitClose so the listener is removed if the
     * collector is cancelled — preventing a memory/socket leak.
     */
    fun messageFlow(): Flow<WsEnvelope> = callbackFlow {
        val listener: (String) -> Unit = { text ->
            runCatching { json.decodeFromString<WsEnvelope>(text) }
                .onSuccess { trySend(it) }
                .onFailure { FirebaseCrashlytics.getInstance().log("WS parse error: ${it.message}") }
        }
        _messageListeners.add(listener)
        awaitClose { _messageListeners.remove(listener) }
    }
}
