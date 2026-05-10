package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
	"github.com/redis/go-redis/v9"
)

// GenerateInvoiceUseCase creates a GST-compliant PDF invoice for a shop.
//
// ADR-001: PDF bytes are returned to the caller (→ Android) but NOT stored server-side.
// Only invoice metadata is persisted in Postgres.
//
// Rate limits:
//   - Daily limit: DailyInvoiceLimit (60) invoices per shop per IST day.
//     Enforced via Redis counter key: invoice_count:{shopID}:{YYYY-MM-DD-IST}
type GenerateInvoiceUseCase struct {
	shops    *infrastructure.ShopRepository
	invoices *infrastructure.InvoiceRepository
	pdfBuild *infrastructure.InvoicePDFBuilder
	subProj  *infrastructure.SubscriptionProjection
	rdb      *redis.Client
}

func NewGenerateInvoiceUseCase(
	shops *infrastructure.ShopRepository,
	invoices *infrastructure.InvoiceRepository,
	pdfBuild *infrastructure.InvoicePDFBuilder,
	subProj *infrastructure.SubscriptionProjection,
	rdb *redis.Client,
) *GenerateInvoiceUseCase {
	return &GenerateInvoiceUseCase{
		shops: shops, invoices: invoices,
		pdfBuild: pdfBuild, subProj: subProj, rdb: rdb,
	}
}

type GenerateInvoiceInput struct {
	UserID             uuid.UUID
	ShopID             uuid.UUID
	CustomerName       string
	CustomerPhone      *string
	Items              []domain.InvoiceLineItem
	PaymentMode        domain.PaymentMode
	Notes              *string
	GoldRateOverride   *float64 // nil = use live rate from pricing service
	SilverRateOverride *float64 // nil = use live rate from pricing service
	LiveGoldRate       float64  // fetched by handler before calling usecase
	LiveSilverRate     float64
	RateSource         domain.RateSource
}

type GenerateInvoiceOutput struct {
	InvoiceID   uuid.UUID
	PDFBytes    []byte
	GeneratedAt time.Time
	RateSource  domain.RateSource
}

func (uc *GenerateInvoiceUseCase) Execute(ctx context.Context, in GenerateInvoiceInput) (*GenerateInvoiceOutput, error) {
	// 1. PREMIUM guard.
	premium, err := uc.subProj.IsPremium(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("subscription check: %w", err)
	}
	if !premium {
		return nil, domain.ErrNotPremium{}
	}

	// 2. Verify shop ownership.
	shop, err := uc.shops.GetByID(ctx, in.ShopID)
	if err != nil || shop == nil || shop.UserID != in.UserID {
		return nil, fmt.Errorf("shop not found or access denied")
	}

	// 3. Daily rate limit via Redis counter (IST day boundary).
	rateLimitKey := fmt.Sprintf("invoice_count:%s:%s",
		in.ShopID, time.Now().In(infrastructure.IST).Format("2006-01-02"))
	count, err := uc.rdb.Incr(ctx, rateLimitKey).Result()
	if err != nil {
		return nil, fmt.Errorf("rate limit incr: %w", err)
	}
	if count == 1 {
		// First invoice of the day: set TTL to 26 hours (covers IST midnight rollover).
		uc.rdb.Expire(ctx, rateLimitKey, 26*time.Hour)
	}
	if count > int64(domain.DailyInvoiceLimit) {
		return nil, domain.ErrDailyLimitExceeded{Limit: domain.DailyInvoiceLimit}
	}

	// 4. Resolve gold and silver rates.
	goldRate := in.LiveGoldRate
	if in.GoldRateOverride != nil {
		goldRate = *in.GoldRateOverride
	}
	silverRate := in.LiveSilverRate
	if in.SilverRateOverride != nil {
		silverRate = *in.SilverRateOverride
	}

	// 5. Compute line item prices.
	items := computeLineItems(in.Items, goldRate)

	// 6. Build the invoice domain object.
	inv := domain.Invoice{
		ShopID:             in.ShopID,
		UserID:             in.UserID,
		CustomerName:       in.CustomerName,
		CustomerPhone:      in.CustomerPhone,
		Items:              items,
		PaymentMode:        in.PaymentMode,
		Notes:              in.Notes,
		GoldRateOverride:   in.GoldRateOverride,
		SilverRateOverride: in.SilverRateOverride,
		RateSource:         in.RateSource,
	}

	// 7. Generate PDF.
	pdfBytes, err := uc.pdfBuild.Build(infrastructure.InvoicePDFInput{
		Invoice:    inv,
		ShopName:   shop.Name,
		ShopAddr:   shop.Address,
		ShopGST:    shop.GSTNumber,
		ShopPhone:  shop.Phone,
		GoldRate:   goldRate,
		SilverRate: silverRate,
	})
	if err != nil {
		return nil, fmt.Errorf("pdf build: %w", err)
	}

	// 8. Persist metadata (no PDF — ADR-001).
	size := len(pdfBytes)
	inv.PDFSizeBytes = &size
	stored, err := uc.invoices.Insert(ctx, inv)
	if err != nil {
		return nil, fmt.Errorf("invoice insert: %w", err)
	}

	return &GenerateInvoiceOutput{
		InvoiceID:   stored.ID,
		PDFBytes:    pdfBytes,
		GeneratedAt: stored.GeneratedAt,
		RateSource:  stored.RateSource,
	}, nil
}

// computeLineItems calculates UnitPrice, TotalPrice, GSTAmount, and NetAmount
// for each line item using the resolved gold rate and 3% GST.
func computeLineItems(raw []domain.InvoiceLineItem, goldRate float64) []domain.InvoiceLineItem {
	out := make([]domain.InvoiceLineItem, len(raw))
	for i, item := range raw {
		unitPrice := goldRate * item.WeightGrams * domain.KaratFactor(item.Karat)
		total := unitPrice + item.MakingCharge
		gst := total * domain.GSTRate
		net := total + gst
		out[i] = domain.InvoiceLineItem{
			Description:  item.Description,
			WeightGrams:  item.WeightGrams,
			Karat:        item.Karat,
			MakingCharge: item.MakingCharge,
			UnitPrice:    unitPrice,
			TotalPrice:   total,
			GSTAmount:    gst,
			NetAmount:    net,
		}
	}
	return out
}
