package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// GetBannerUploadURLUseCase generates a presigned S3 PUT URL for a shop banner.
//
// Two-phase upload flow:
//  1. Client calls this endpoint → receives presigned PUT URL + objectKey.
//  2. Client PUTs image bytes directly to S3 (backend never handles the binary).
//  3. Client calls ConfirmBannerUploadUseCase with the objectKey to trigger
//     Safe Search moderation and persist the CDN URL on the shop record.
//
// Only PREMIUM users with a registered shop may request an upload URL.
// FREE-tier callers receive ErrNotPremium.
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

// BannerUploadURLOutput carries the presigned URL and S3 object key.
// The client must echo objectKey back to ConfirmBannerUploadUseCase.
type BannerUploadURLOutput struct {
	PresignedURL string
	ObjectKey    string
}

// Execute validates shop ownership, checks PREMIUM status, and returns a
// presigned PUT URL.
//
//  1. PREMIUM guard via SubscriptionProjection (Redis read model).
//  2. Resolve shop owned by userID — must exist before a banner upload is allowed.
//  3. Generate collision-safe object key: banners/{shopID}/{uuid}.jpg
//     The uuid suffix prevents CDN serving stale content when the banner is replaced.
//  4. Presign via S3Client.
func (uc *GetBannerUploadURLUseCase) Execute(ctx context.Context, userID uuid.UUID) (*BannerUploadURLOutput, error) {
	premium, err := uc.subProj.IsPremium(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("subscription check: %w", err)
	}
	if !premium {
		return nil, domain.ErrNotPremium{}
	}

	shop, err := uc.shops.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get shop: %w", err)
	}
	if shop == nil {
		return nil, fmt.Errorf("no shop registered for user %s", userID)
	}

	objectKey := fmt.Sprintf("banners/%s/%s.jpg", shop.ID, uuid.New())

	url, err := uc.s3.PresignBannerUpload(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}

	return &BannerUploadURLOutput{PresignedURL: url, ObjectKey: objectKey}, nil
}
