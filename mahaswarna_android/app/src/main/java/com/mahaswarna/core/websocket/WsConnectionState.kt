package com.mahaswarna.core.websocket

/**
 * WsClient connection state machine.
 *
 * Transitions:
 *   Connecting → Connected
 *   Connected  → Reconnecting (on transient failure — IOException, timeout)
 *   Connected  → Disconnected (explicit disconnect() call)
 *   Reconnecting → Connected  (backoff retry succeeded)
 *   Reconnecting → Error      (non-retryable failure during backoff)
 *   Any        → Error        (non-retryable failure — HandshakeException / cert mismatch)
 *
 * StaleRateBanner rules:
 *   Error        → show IMMEDIATELY (no 30s grace)
 *   Reconnecting / Disconnected → start 30s timer; show if still not Connected
 *   Connected    → hide; cancel timer
 *
 * Error is TERMINAL — WsClient does NOT auto-retry. Requires explicit
 * user retry or app restart to recover.
 */
sealed class WsConnectionState {
    data object Connecting    : WsConnectionState()
    data object Connected     : WsConnectionState()
    data object Reconnecting  : WsConnectionState()
    data object Disconnected  : WsConnectionState()
    data class  Error(val cause: Throwable?) : WsConnectionState()

    /** True when this state should eventually trigger StaleRateBanner. */
    val isStaleCondition: Boolean
        get() = this !is Connected
}
