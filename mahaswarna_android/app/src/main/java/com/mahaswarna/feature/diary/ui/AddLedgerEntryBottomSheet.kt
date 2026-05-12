package com.mahaswarna.feature.diary.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp

/**
 * Bottom sheet to add a manual credit or debit ledger entry for a customer.
 * Triggered from [CustomerLedgerDetailScreen].
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AddLedgerEntryBottomSheet(
    customerId: String,
    onDismiss: () -> Unit,
    onSave: (type: String, amountInr: Double, description: String) -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartialExpansion = true)
    var type by remember { mutableStateOf("credit") }
    var amountStr by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var amountError by remember { mutableStateOf<String?>(null) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState       = sheetState,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 24.dp, end = 24.dp, top = 8.dp, bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Text("Add Ledger Entry", style = androidx.compose.material3.MaterialTheme.typography.titleLarge)

            // Credit / Debit toggle
            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                FilterChip(
                    selected = type == "credit",
                    onClick  = { type = "credit" },
                    label    = { Text("Credit (received)") },
                )
                FilterChip(
                    selected = type == "debit",
                    onClick  = { type = "debit" },
                    label    = { Text("Debit (owed)") },
                )
            }

            OutlinedTextField(
                value         = amountStr,
                onValueChange = {
                    amountStr   = it
                    amountError = null
                },
                label         = { Text("Amount (₹)") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                isError       = amountError != null,
                supportingText = amountError?.let { { Text(it) } },
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )

            OutlinedTextField(
                value         = description,
                onValueChange = { description = it },
                label         = { Text("Description (optional)") },
                modifier      = Modifier.fillMaxWidth(),
                maxLines      = 2,
            )

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp, Alignment.End),
            ) {
                TextButton(onClick = onDismiss) { Text("Cancel") }
                Button(onClick = {
                    val amount = amountStr.toDoubleOrNull()
                    if (amount == null || amount <= 0) {
                        amountError = "Enter a valid amount"
                        return@Button
                    }
                    onSave(type, amount, description.trim())
                }) {
                    Text("Save")
                }
            }
        }
    }
}

/**
 * Bottom sheet for adding a new customer from [DiaryScreen].
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AddCustomerBottomSheet(
    onDismiss: () -> Unit,
    onSave: (name: String, phone: String, address: String, gst: String, notes: String) -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartialExpansion = true)
    var name by remember { mutableStateOf("") }
    var phone by remember { mutableStateOf("") }
    var address by remember { mutableStateOf("") }
    var gst by remember { mutableStateOf("") }
    var notes by remember { mutableStateOf("") }
    var nameError by remember { mutableStateOf<String?>(null) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState       = sheetState,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 24.dp, end = 24.dp, top = 8.dp, bottom = 32.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("New Customer", style = androidx.compose.material3.MaterialTheme.typography.titleLarge)

            OutlinedTextField(
                value         = name,
                onValueChange = { name = it; nameError = null },
                label         = { Text("Name *") },
                isError       = nameError != null,
                supportingText = nameError?.let { { Text(it) } },
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )
            OutlinedTextField(
                value         = phone,
                onValueChange = { phone = it },
                label         = { Text("Phone") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )
            OutlinedTextField(
                value         = address,
                onValueChange = { address = it },
                label         = { Text("Address") },
                modifier      = Modifier.fillMaxWidth(),
                maxLines      = 2,
            )
            OutlinedTextField(
                value         = gst,
                onValueChange = { gst = it },
                label         = { Text("GSTIN (optional)") },
                modifier      = Modifier.fillMaxWidth(),
                singleLine    = true,
            )

            Spacer(modifier = Modifier.height(4.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp, Alignment.End),
            ) {
                TextButton(onClick = onDismiss) { Text("Cancel") }
                Button(onClick = {
                    if (name.isBlank()) {
                        nameError = "Name is required"
                        return@Button
                    }
                    onSave(name.trim(), phone.trim(), address.trim(), gst.trim(), notes.trim())
                }) {
                    Text("Save")
                }
            }
        }
    }
}
