package com.mahaswarna.feature.marketplace.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mahaswarna.feature.marketplace.data.MarketplaceRepository
import com.mahaswarna.feature.marketplace.domain.ConfirmBannerUseCase
import com.mahaswarna.feature.marketplace.domain.GetBannerUploadUrlUseCase
import com.mahaswarna.feature.marketplace.domain.RegisterShopInput
import com.mahaswarna.feature.marketplace.domain.RegisterShopUseCase
import com.mahaswarna.feature.marketplace.domain.Shop
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import javax.inject.Inject
import javax.inject.Named

// ── UI State ──────────────────────────────────────────────────────────────────

sealed class ShopUiState {
    data object Loading : ShopUiState()
    data object NoShop : ShopUiState()
    data class HasShop(val shop: Shop) : ShopUiState()
    data class Error(val message: String) : ShopUiState()
}

sealed class ShopEvent {
    data class ShowError(val message: String) : ShopEvent()
    data class ShowSuccess(val message: String) : ShopEvent()
    data object RegistrationComplete : ShopEvent()
    data object BannerUploaded : ShopEvent()
    data object QuotaExceeded : ShopEvent()
}

// ── ViewModel ─────────────────────────────────────────────────────────────────

@HiltViewModel
class ShopViewModel @Inject constructor(
    private val repository: MarketplaceRepository,
    private val registerShopUseCase: RegisterShopUseCase,
    private val getBannerUploadUrlUseCase: GetBannerUploadUrlUseCase,
    private val confirmBannerUseCase: ConfirmBannerUseCase,
    // The @Named("s3") OkHttpClient has no AuthInterceptor — required for S3
    // presigned URL uploads. The primary client adds a Bearer token header that
    // S3 presigned URLs reject with 400 SignatureDoesNotMatch.
    @Named("s3") private val s3HttpClient: OkHttpClient,
) : ViewModel() {

    private val _uiState = MutableStateFlow<ShopUiState>(ShopUiState.Loading)
    val uiState: StateFlow<ShopUiState> = _uiState.asStateFlow()

    private val _events = MutableStateFlow<ShopEvent?>(null)
    val events: StateFlow<ShopEvent?> = _events.asStateFlow()

    fun eventConsumed() { _events.value = null }

    init { loadShops() }

    // ── Shop listing ──────────────────────────────────────────────────────────

    fun loadShops() {
        viewModelScope.launch {
            _uiState.value = ShopUiState.Loading
            runCatching { repository.listShops() }
                .onSuccess { shops ->
                    _uiState.value = if (shops.isEmpty()) ShopUiState.NoShop
                                     else ShopUiState.HasShop(shops.first())
                }
                .onFailure { e ->
                    _uiState.value = ShopUiState.Error(e.message ?: "Failed to load shops")
                }
        }
    }

    // ── Registration ──────────────────────────────────────────────────────────

    fun registerShop(input: RegisterShopInput) {
        viewModelScope.launch {
            _uiState.value = ShopUiState.Loading
            when (val result = registerShopUseCase(input)) {
                is RegisterShopUseCase.RegisterResult.Success -> {
                    _uiState.value = ShopUiState.HasShop(result.shop)
                    _events.value  = ShopEvent.RegistrationComplete
                }
                is RegisterShopUseCase.RegisterResult.NotPremium -> {
                    _uiState.value = ShopUiState.NoShop
                    _events.value  = ShopEvent.ShowError("Upgrade to Premium to register a shop.")
                }
                is RegisterShopUseCase.RegisterResult.AlreadyRegistered -> {
                    _events.value = ShopEvent.ShowError("You already have a registered shop.")
                    loadShops()
                }
                is RegisterShopUseCase.RegisterResult.InvalidGstin -> {
                    _uiState.value = ShopUiState.NoShop
                    _events.value  = ShopEvent.ShowError(result.message)
                }
                is RegisterShopUseCase.RegisterResult.Failure -> {
                    _uiState.value = ShopUiState.NoShop
                    _events.value  = ShopEvent.ShowError(result.message)
                }
            }
        }
    }

    // ── Banner upload: presign → S3 PUT → confirm ─────────────────────────────

    fun uploadBanner(shopId: String, imageBytes: ByteArray, contentType: String = "image/jpeg") {
        viewModelScope.launch {
            // Step 1: get presigned URL
            val presign = getBannerUploadUrlUseCase(shopId).getOrElse { e ->
                _events.value = ShopEvent.ShowError("Failed to get upload URL: ${e.message}")
                return@launch
            }

            // Step 2: PUT directly to S3 via s3HttpClient (no Auth header — presigned URL is self-contained)
            val s3Request = Request.Builder()
                .url(presign.uploadUrl)
                .put(imageBytes.toRequestBody(presign.contentType.toMediaTypeOrNull()))
                .header("Content-Type", presign.contentType)
                .build()

            val s3Response = runCatching { s3HttpClient.newCall(s3Request).execute() }
                .getOrElse { e ->
                    _events.value = ShopEvent.ShowError("Upload failed: ${e.message}")
                    return@launch
                }

            if (!s3Response.isSuccessful) {
                _events.value = ShopEvent.ShowError("S3 upload failed (HTTP ${s3Response.code})")
                s3Response.close()
                return@launch
            }
            s3Response.close()

            // Step 3: confirm with backend — triggers async image moderation
            confirmBannerUseCase(shopId, presign.objectKey).onFailure { e ->
                _events.value = ShopEvent.ShowError("Banner confirm failed: ${e.message}")
                return@launch
            }

            _events.value = ShopEvent.BannerUploaded
            loadShops()  // poll for updated banner_url after moderation
        }
    }

    // ── Shop refresh ──────────────────────────────────────────────────────────

    fun refreshShop(shopId: String) {
        viewModelScope.launch {
            runCatching { repository.getShop(shopId) }
                .onSuccess { shop -> _uiState.value = ShopUiState.HasShop(shop) }
                .onFailure { e -> _events.value = ShopEvent.ShowError(e.message ?: "Refresh failed") }
        }
    }
}
