package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// GetBannerUploadURLUseCase generates a presigned S3 PUT URL for a shop banner.
// The client must PUT the image directly to S3, then call ConfirmBannerUploadUseCase.
type GetBannerUploadURLUseCase struct {
	shops   *infrastructure.ShopRepository
	s3      *infrastructure.S3Client
	subProj *infrastructure.SubscriptionProjection
}

func NewGetBannerUploadURLUseCase(
	shops *infrastructure.ShopRepository,
	s3 *infrastructure.S3Client,
	subProj *infrastructure.SubscriptionProjection,
) *GetBannerUploadURLUseCase {
	return &GetBannerUploadURLUseCase{shops: shops, s3: s3, subProj: subProj}
}

type BannerUploadURLOutput struct {
	PresignedURL string
	ObjectKey    string
}

// Execute validates shop ownership, then returns a presigned PUT URL.
func (uc *GetBannerUploadURLUseCase) Execute(ctx context.Context, userID uuid.UUID) (*BannerUploadURLOutput, error) {
	// 1. PREMIUM guard.
	premium, err := uc.subProj.IsPremium(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("subscription check: %w", err)
	}
	if !premium {
		return nil, domain.ErrNotPremium{}
	}

	// 2. Ensure shop exists.
	shop, err := uc.shops.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get shop: %w", err)
	}
	if shop == nil {
		return nil, fmt.Errorf("no shop registered for user %s", userID)
	}

	// 3. Object key: banners/{shopID}/{uuid}.jpg — uuid ensures no caching collisions on replace.
	objectKey := fmt.Sprintf("banners/%s/%s.jpg", shop.ID, uuid.New())

	url, err := uc.s3.PresignBannerUpload(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}
	return &BannerUploadURLOutput{PresignedURL: url, ObjectKey: objectKey}, nil
}

// ───────────────────────────────────────────────────────────────────────────────

// ConfirmBannerUploadUseCase is called after the client has PUT the image to S3.
// It runs Safe Search moderation, and if it passes, updates the shop record.
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

type ConfirmBannerInput struct {
	UserID    uuid.UUID
	ShopID    uuid.UUID
	ObjectKey string
	S3Bucket  string // needed to form gs:// URI for Vision API
}

// Execute verifies the upload, runs moderation, and updates the shop record.
// If moderation fails, the uploaded S3 object is deleted.
func (uc *ConfirmBannerUploadUseCase) Execute(ctx context.Context, in ConfirmBannerInput) (*domain.Shop, error) {
	// 1. Verify the shop belongs to this user.
	shop, err := uc.shops.GetByID(ctx, in.ShopID)
	if err != nil || shop == nil || shop.UserID != in.UserID {
		return nil, fmt.Errorf("shop not found or access denied")
	}

	// 2. Verify object exists in S3.
	exists, err := uc.s3.ObjectExists(ctx, in.ObjectKey)
	if err != nil || !exists {
		return nil, fmt.Errorf("uploaded object not found in S3: %s", in.ObjectKey)
	}

	// 3. Run Safe Search moderation.
	gcsURI := fmt.Sprintf("gs://%s/%s", in.S3Bucket, in.ObjectKey)
	verdict, err := uc.moderation.Moderate(ctx, gcsURI)
	if err != nil {
		// Moderation error: delete upload and reject.
		_ = uc.s3.DeleteObject(ctx, in.ObjectKey)
		return nil, fmt.Errorf("moderation error: %w", err)
	}
	if verdict == infrastructure.ModerationFail {
		_ = uc.s3.DeleteObject(ctx, in.ObjectKey)
		return nil, fmt.Errorf("banner rejected by content moderation")
	}

	// 4. Delete old banner if replacing.
	if shop.BannerObjectKey != nil && *shop.BannerObjectKey != "" {
		_ = uc.s3.DeleteObject(ctx, *shop.BannerObjectKey) // best-effort
	}

	// 5. Update shop record.
	cdnURL := uc.cdn.Build(in.ObjectKey)
	if err := uc.shops.UpdateBanner(ctx, in.ShopID, cdnURL, in.ObjectKey); err != nil {
		return nil, fmt.Errorf("update banner: %w", err)
	}

	shop.BannerURL = &cdnURL
	shop.BannerObjectKey = &in.ObjectKey
	return shop, nil
}
