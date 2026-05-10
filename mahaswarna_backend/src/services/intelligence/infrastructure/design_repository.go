package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/intelligence/domain"
)

// DesignRepository provides read/write access to the design_catalog table.
// view_count is NOT incremented here; use ViewCountCache (Redis) for that.
type DesignRepository struct {
	pool *pgxpool.Pool
}

func NewDesignRepository(pool *pgxpool.Pool) *DesignRepository {
	return &DesignRepository{pool: pool}
}

// Search performs a full-text search using the tsvector GIN index.
// Falls back to plain ilike on empty query (browse mode).
func (r *DesignRepository) Search(ctx context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
	const maxPageSize = 50
	if params.PageSize <= 0 || params.PageSize > maxPageSize {
		params.PageSize = maxPageSize
	}
	if params.Page < 1 {
		params.Page = 1
	}
	offset := (params.Page - 1) * params.PageSize

	// Build WHERE clauses dynamically.
	var whereClauses []string
	var args []any
	argIdx := 1

	if params.Query != "" {
		// tsquery: convert space-separated words to prefix-match websearch query.
		whereClauses = append(whereClauses,
			fmt.Sprintf("search_vector @@ plainto_tsquery('english', $%d)", argIdx))
		args = append(args, params.Query)
		argIdx++
	}
	if params.Region != "" {
		whereClauses = append(whereClauses,
			fmt.Sprintf("(region IS NULL OR region = $%d)", argIdx))
		args = append(args, params.Region)
		argIdx++
	}
	if params.MetalType != "" {
		whereClauses = append(whereClauses,
			fmt.Sprintf("(metal_type = $%d OR metal_type = 'both')", argIdx))
		args = append(args, string(params.MetalType))
		argIdx++
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query.
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM design_catalog %s`, where)
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("design search count: %w", err)
	}

	// Order: FTS rank when query present, view_count DESC otherwise.
	orderBy := "view_count DESC"
	if params.Query != "" {
		orderBy = fmt.Sprintf(
			"ts_rank(search_vector, plainto_tsquery('english', $%d)) DESC, view_count DESC",
			argIdx)
		args = append(args, params.Query)
		argIdx++
	}

	dataSQL := fmt.Sprintf(`
		SELECT id, title, description, category, style, region, metal_type,
		       image_url, tags, view_count, created_at, updated_at
		FROM design_catalog
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argIdx, argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("design search query: %w", err)
	}
	defer rows.Close()

	designs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Design, error) {
		var d domain.Design
		return d, row.Scan(
			&d.ID, &d.Title, &d.Description, &d.Category, &d.Style,
			&d.Region, &d.MetalType, &d.ImageURL, &d.Tags,
			&d.ViewCount, &d.CreatedAt, &d.UpdatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("design search scan: %w", err)
	}

	totalPages := (total + params.PageSize - 1) / params.PageSize
	return &domain.SearchResult{
		Designs:    designs,
		TotalCount: total,
		Page:       params.Page,
		TotalPages: totalPages,
	}, nil
}

// GetByID returns a single design by ID.
func (r *DesignRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Design, error) {
	const q = `
		SELECT id, title, description, category, style, region, metal_type,
		       image_url, tags, view_count, created_at, updated_at
		FROM design_catalog
		WHERE id = $1
	`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("design get by id: %w", err)
	}
	defer rows.Close()

	d, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (domain.Design, error) {
		var d domain.Design
		return d, row.Scan(
			&d.ID, &d.Title, &d.Description, &d.Category, &d.Style,
			&d.Region, &d.MetalType, &d.ImageURL, &d.Tags,
			&d.ViewCount, &d.CreatedAt, &d.UpdatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("design get by id scan: %w", err)
	}
	return &d, nil
}

// BulkAddViewCounts increments view_count in bulk from the Redis flush job.
// Uses a single UPDATE … CASE statement to avoid N round-trips.
func (r *DesignRepository) BulkAddViewCounts(ctx context.Context, counts map[uuid.UUID]int64) error {
	if len(counts) == 0 {
		return nil
	}

	ids := make([]uuid.UUID, 0, len(counts))
	deltas := make([]int64, 0, len(counts))
	for id, delta := range counts {
		ids = append(ids, id)
		deltas = append(deltas, delta)
	}

	// Build: UPDATE design_catalog SET view_count = view_count + deltas[i] WHERE id = ids[i]
	// Batch via unnest for efficiency.
	const q = `
		UPDATE design_catalog AS d
		SET view_count = d.view_count + v.delta
		FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::bigint[]) AS delta) AS v
		WHERE d.id = v.id
	`
	if _, err := r.pool.Exec(ctx, q, ids, deltas); err != nil {
		return fmt.Errorf("bulk add view counts: %w", err)
	}
	return nil
}
