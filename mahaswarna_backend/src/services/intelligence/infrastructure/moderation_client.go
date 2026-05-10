package infrastructure

import (
	"context"
	"fmt"

	vision "cloud.google.com/go/vision/apiv1"
	visionpb "cloud.google.com/go/vision/apiv1/visionpb"
)

// ModerationVerdict is the result of Safe Search moderation.
type ModerationVerdict int

const (
	ModerationPass ModerationVerdict = iota
	ModerationFail
)

// ModerationClient uses Google Cloud Vision Safe Search to reject
// inappropriate banner images before they go live.
// CREDENTIALS: uses Application Default Credentials (GOOGLE_APPLICATION_CREDENTIALS env var).
type ModerationClient struct {
	client *vision.ImageAnnotatorClient
}

func NewModerationClient(ctx context.Context) (*ModerationClient, error) {
	c, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("vision client: %w", err)
	}
	return &ModerationClient{client: c}, nil
}

func (m *ModerationClient) Close() {
	_ = m.client.Close()
}

// Moderate fetches and analyses the image at the given GCS URL.
// gcsBannerURI must be "gs://bucket/key" format (not the CDN URL).
// Returns ModerationFail if any likelihood field is LIKELY or VERY_LIKELY.
func (m *ModerationClient) Moderate(ctx context.Context, gcsBannerURI string) (ModerationVerdict, error) {
	req := &visionpb.AnnotateImageRequest{
		Image: &visionpb.Image{
			Source: &visionpb.ImageSource{GcsImageUri: gcsBannerURI},
		},
		Features: []*visionpb.Feature{
			{Type: visionpb.Feature_SAFE_SEARCH_DETECTION},
		},
	}

	resp, err := m.client.AnnotateImage(ctx, req)
	if err != nil {
		return ModerationFail, fmt.Errorf("vision annotate: %w", err)
	}
	if resp.Error != nil {
		return ModerationFail, fmt.Errorf("vision api error: %s", resp.Error.Message)
	}

	ss := resp.SafeSearchAnnotation
	if ss == nil {
		// No annotation means Vision couldn't parse it — reject.
		return ModerationFail, nil
	}

	// Reject if adult, violence, or racy content is LIKELY (4) or VERY_LIKELY (5).
	const threshold = visionpb.Likelihood_LIKELY
	if ss.Adult >= threshold || ss.Violence >= threshold || ss.Racy >= threshold {
		return ModerationFail, nil
	}
	return ModerationPass, nil
}
