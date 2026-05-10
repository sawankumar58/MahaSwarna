package infrastructure

import (
	"context"
	"fmt"
	"log/slog"
)

// RepopulateSubscriptionProjection re-seeds the Redis subscription projection
// after a pgnotify reconnect. It queries all shops via ShopRepository.ListAll
// and calls SubscriptionProjection.RefreshFromSource for each owner to recover
// any events missed during the reconnect window.
//
// Design:
//   - Fail-open per shop: one failed refresh is logged and collected but does not
//     abort the rest of the population.
//   - RefreshFromSource sets tier to PREMIUM optimistically on cache miss; a
//     subsequent subscription_expired NOTIFY will correct it to FREE if needed.
//
// This satisfies the NOTIFY reconnect invariant documented in ARCHITECTURE.md:
// every onReconnect callback must re-run the startup catch-up query so that
// events missed during the reconnect window do not leave the projection stale.
func RepopulateSubscriptionProjection(
	ctx context.Context,
	proj *SubscriptionProjection,
	shops *ShopRepository,
) error {
	allShops, err := shops.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("repopulate: list shops: %w", err)
	}

	var errs []error
	for _, shop := range allShops {
		if err := proj.RefreshFromSource(ctx, shop.UserID); err != nil {
			slog.Warn("repopulate: projection refresh failed",
				"userID", shop.UserID, "err", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("repopulate: %d refresh errors (first: %w)", len(errs), errs[0])
	}
	slog.Info("subscription projection repopulated", "shops", len(allShops))
	return nil
}
