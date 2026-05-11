package com.mahaswarna.core.websocket

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

/**
 * Wire format for WebSocket messages from the pricing service.
 * channel: "rates" | "alerts"
 * payload: channel-specific JSON; deserialized downstream by WsChannelRouter.
 */
@Serializable
data class WsEnvelope(
    val channel: String,
    val payload: JsonElement,
)
