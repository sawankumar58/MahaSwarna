package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// ConfirmBannerUploadUseCase is called after the client has PUT the image to S3.
//
// Responsibilities:
//  1. Verify object was actually uploaded (HEAD check via S3Client).
//  2. Run Google Vision Safe Search moderation — delete object and reject on fail.
//  3. Delete the previous banner object from S3 (best-effort, non-fatal).
//  4. Persist new CDN URL and objectKey on the shop record.
type ConfirmBannerUploadUseCase struct {
	shops      *infrastructure.ShopRepository
	s3         *infrastructure.S3Client
	moderation *infrastructure.ModerationClient
	cdn        *infrastructure.CDNURLBuilder
}

func NewConfirmBannerUploadUseCase(
	shops *infrastructure.ShopRepository,
	s3 *infrastructure.S3Client,
	moderation *infrastructure.ModerationClient,
	cdn *infrastructure.CDNURLBuilder,
) *ConfirmBannerUploadUseCase {
	return &ConfirmBannerUploadUseCase{shops: shops, s3: s3, moderation: moderation, cdn: cdn}
}

// ConfirmBannerInput carries all fields needed to verify and commit an upload.
type ConfirmBannerInput struct {
	UserID    uuid.UUID
	ShopID    uuid.UUID
	ObjectKey string // returned by GetBannerUploadURLUseCase
	S3Bucket  string // forms the gs:// URI required by Vision API
}

// Execute verifies the upload, runs content moderation, and persists the CDN URL.
//
//  1. Ownership check — shop must belong to the calling user.
//  2. HEAD check — reject if S3 object not found (upload never happened).
//  3. Safe Search moderation via ModerationClient (Vision API).
//     On error or ModerationFail: delete S3 object and return error.
//  4. Delete previous banner object if present (CDN cache busting, best-effort).
//  5. Persist shop.banner_url and shop.banner_object_key in Postgres.
//  6. Return updated Shop (avoids a second DB round-trip).
func (uc *ConfirmBannerUploadUseCase) Execute(ctx context.Context, in ConfirmBannerInput) (*domain.Shop, error) {
	shop, err := uc.shops.GetByID(ctx, in.ShopID)
	if err != nil || shop == nil || shop.UserID != in.UserID {
		return nil, fmt.Errorf("shop not found or access denied")
	}

	exists, err := uc.s3.ObjectExists(ctx, in.ObjectKey)
	if err != nil || !exists {
		return nil, fmt.Errorf("uploaded object not found in S3: %s", in.ObjectKey)
	}

	gcsURI := fmt.Sprintf("gs://%s/%s", in.S3Bucket, in.ObjectKey)
	verdict, err := uc.moderation.Moderate(ctx, gcsURI)
	if err != nil {
		_ = uc.s3.DeleteObject(ctx, in.ObjectKey)
		return nil, fmt.Errorf("moderation error: %w", err)
	}
	if verdict == infrastructure.ModerationFail {
		_ = uc.s3.DeleteObject(ctx, in.ObjectKey)
		return nil, fmt.Errorf("banner rejected by content moderation")
	}

	if shop.BannerObjectKey != nil && *shop.BannerObjectKey != "" {
		_ = uc.s3.DeleteObject(ctx, *shop.BannerObjectKey) // best-effort
	}

	cdnURL := uc.cdn.Build(in.ObjectKey)
	if err := uc.shops.UpdateBanner(ctx, in.ShopID, cdnURL, in.ObjectKey); err != nil {
		return nil, fmt.Errorf("update banner: %w", err)
	}

	shop.BannerURL = &cdnURL
	shop.BannerObjectKey = &in.ObjectKey
	return shop, nil
}
