package com.mahaswarna.feature.diary.domain

/**
 * Domain model for a jeweller's customer in the local address book.
 *
 * [gstNumber] is optional; stored as empty string when absent.
 * Derived from [com.mahaswarna.local.entity.CustomerEntity] via mapper.
 */
data class Customer(
    val id: String,
    val name: String,
    val phone: String = "",
    val address: String = "",
    val gstNumber: String = "",
    val notes: String = "",
    /** Epoch milliseconds (device local time). */
    val createdAt: Long,
    val updatedAt: Long,
)
