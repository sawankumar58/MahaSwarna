package com.mahaswarna.feature.marketplace.ui

import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.feature.marketplace.data.InvoiceLineItemDto
import com.mahaswarna.feature.marketplace.data.InvoiceRequest
import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import com.mahaswarna.feature.marketplace.domain.GenerateInvoiceUseCase
import com.mahaswarna.feature.marketplace.domain.Shop
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.io.File
import javax.inject.Inject

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class BillPrintUiState {
    data object Idle : BillPrintUiState()
    data object GeneratingPdf : BillPrintUiState()
    data class PdfReady(val shareIntent: Intent) : BillPrintUiState()
    data object QuotaExceeded : BillPrintUiState()
    data class Error(val message: String) : BillPrintUiState()
    data object NoShopRegistered : BillPrintUiState()
}

data class LineItemForm(
    val description: String = "",
    val weightGrams: String = "",
    val karat: Int = 22,
    val makingCharge: String = "0",
)

// ── ViewModel ─────────────────────────────────────────────────────────────────

@HiltViewModel
class BillPrintViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val generateInvoiceUseCase: GenerateInvoiceUseCase,
    private val repository: MarketplaceRepository,
    @ApplicationContext private val context: Context,
) : ViewModel() {

    /**
     * Nav args for nullable Double must be read from [SavedStateHandle] via getString
     * then parsed — Compose Navigation serializes nullable Doubles as strings in the bundle.
     * Reading via `arguments?.getDouble("goldRate")` returns 0.0 (not null) for absent keys.
     */
    val goldRate: Double?   = savedStateHandle.get<String>("goldRate")?.toDoubleOrNull()
    val silverRate: Double? = savedStateHandle.get<String>("silverRate")?.toDoubleOrNull()

    private val _uiState = MutableStateFlow<BillPrintUiState>(BillPrintUiState.Idle)
    val uiState: StateFlow<BillPrintUiState> = _uiState.asStateFlow()

    private var cachedShop: Shop? = null

    init { loadShop() }

    private fun loadShop() {
        viewModelScope.launch {
            runCatching { repository.listShops() }
                .onSuccess { shops ->
                    cachedShop = shops.firstOrNull()
                    if (shops.isEmpty()) _uiState.value = BillPrintUiState.NoShopRegistered
                }
                .onFailure { /* non-fatal on init; VM will surface NoShopRegistered on generate */ }
        }
    }

    fun generateInvoice(
        customerName: String,
        customerPhone: String?,
        lineItems: List<LineItemForm>,
        paymentMode: String,
        notes: String?,
        goldRate: Double?,
        silverRate: Double?,
    ) {
        val shop = cachedShop ?: run {
            _uiState.value = BillPrintUiState.NoShopRegistered
            return
        }

        val items = lineItems.mapNotNull { form ->
            val weight = form.weightGrams.toDoubleOrNull()?.takeIf { it > 0 }
                ?: return@mapNotNull null
            InvoiceLineItemDto(
                description  = form.description.ifBlank { "Jewellery item" },
                weightGrams  = weight,
                karat        = form.karat,
                makingCharge = form.makingCharge.toDoubleOrNull() ?: 0.0,
            )
        }

        if (items.isEmpty()) {
            _uiState.value = BillPrintUiState.Error("Add at least one item with valid weight")
            return
        }

        viewModelScope.launch {
            _uiState.value = BillPrintUiState.GeneratingPdf
            when (val result = generateInvoiceUseCase(
                InvoiceRequest(
                    shopId             = shop.id,
                    customerName       = customerName.trim(),
                    customerPhone      = customerPhone?.trim()?.takeIf { it.isNotBlank() },
                    items              = items,
                    paymentMode        = paymentMode,
                    notes              = notes?.trim()?.takeIf { it.isNotBlank() },
                    goldRateOverride   = goldRate,
                    silverRateOverride = silverRate,
                )
            )) {
                is GenerateInvoiceUseCase.InvoiceResult.Success ->
                    _uiState.value = BillPrintUiState.PdfReady(buildShareIntent(result.pdfBytes))
                is GenerateInvoiceUseCase.InvoiceResult.QuotaExceeded ->
                    _uiState.value = BillPrintUiState.QuotaExceeded
                is GenerateInvoiceUseCase.InvoiceResult.Failure ->
                    _uiState.value = BillPrintUiState.Error(result.message)
            }
        }
    }

    fun reset() { _uiState.value = BillPrintUiState.Idle }

    /**
     * Writes PDF bytes to the invoices/ subdirectory of cacheDir and returns a
     * share Intent via FileProvider.
     *
     * The file is written to getCacheDir()/invoices/ to match the FileProvider
     * mapping in file_paths.xml:
     *   <cache-path name="invoice_pdfs" path="invoices/" />
     *
     * mkdirs() is called on the parent directory to handle first run after install.
     */
    private fun buildShareIntent(pdfBytes: ByteArray): Intent {
        val invoiceDir = File(context.cacheDir, "invoices").also { it.mkdirs() }
        val file = File(invoiceDir, "invoice_${System.currentTimeMillis()}.pdf")
        file.writeBytes(pdfBytes)
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
        return Intent(Intent.ACTION_SEND).apply {
            type     = "application/pdf"
            putExtra(Intent.EXTRA_STREAM, uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
    }
}
