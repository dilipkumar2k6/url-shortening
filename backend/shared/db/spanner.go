package db

import (
	"context"
	"os"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserHistoryRecord struct {
	ID        string    `spanner:"id" json:"id"`
	LongURL   string    `spanner:"long_url" json:"long_url"`
	CreatedAt time.Time `spanner:"created_at" json:"created_at"`
}

type SpannerRepo interface {
	SaveURL(ctx context.Context, slug string, longURL string, userID string) error
	UpdateURL(ctx context.Context, slug string, longURL string, userID string) error
	DeleteURL(ctx context.Context, slug string, userID string) error
	GetURLStale(ctx context.Context, slug string, staleness time.Duration) (string, error)
	GetUserHistory(ctx context.Context, userID string) ([]UserHistoryRecord, error)
}

type spannerRepo struct {
	client *spanner.Client
}

func NewSpannerRepo(ctx context.Context, db string) (SpannerRepo, error) {
	var opts []option.ClientOption
	if host := os.Getenv("SPANNER_EMULATOR_HOST"); host != "" {
		opts = []option.ClientOption{
			option.WithEndpoint(host),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			option.WithoutAuthentication(),
		}
	}

	client, err := spanner.NewClient(ctx, db, opts...)
	if err != nil {
		return nil, err
	}
	return &spannerRepo{client: client}, nil
}

func (r *spannerRepo) SaveURL(ctx context.Context, slug string, longURL string, userID string) error {
	var userVal interface{} = userID
	if userID == "" {
		userVal = spanner.NullString{}
	}
	_, err := r.client.Apply(ctx, []*spanner.Mutation{
		spanner.Insert("url_mappings",
			[]string{"id", "long_url", "user_id", "created_at"},
			[]interface{}{slug, longURL, userVal, spanner.CommitTimestamp}),
	})
	return err
}

func (r *spannerRepo) UpdateURL(ctx context.Context, slug string, longURL string, userID string) error {
	if userID == "" {
		return spanner.ToSpannerError(context.DeadlineExceeded) // Or some other error indicating unauthorized
	}

	// Use a ReadWriteTransaction to ensure atomic ownership check and update.
	_, err := r.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		// Check ownership
		row, err := txn.ReadRow(ctx, "url_mappings", spanner.Key{slug}, []string{"user_id"})
		if err != nil {
			return err
		}
		var dbUserID spanner.NullString
		if err := row.Column(0, &dbUserID); err != nil {
			return err
		}

		if !dbUserID.Valid || dbUserID.StringVal != userID {
			return spanner.ToSpannerError(context.DeadlineExceeded) // Unauthorized
		}

		// Update
		return txn.BufferWrite([]*spanner.Mutation{
			spanner.Update("url_mappings", []string{"id", "long_url"}, []interface{}{slug, longURL}),
		})
	})
	return err
}

func (r *spannerRepo) DeleteURL(ctx context.Context, slug string, userID string) error {
	if userID == "" {
		return spanner.ToSpannerError(context.DeadlineExceeded) // Unauthorized
	}

	_, err := r.client.ReadWriteTransaction(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		// Check ownership
		row, err := txn.ReadRow(ctx, "url_mappings", spanner.Key{slug}, []string{"user_id"})
		if err != nil {
			return err
		}
		var dbUserID spanner.NullString
		if err := row.Column(0, &dbUserID); err != nil {
			return err
		}

		if !dbUserID.Valid || dbUserID.StringVal != userID {
			return spanner.ToSpannerError(context.DeadlineExceeded) // Unauthorized
		}

		// Delete
		return txn.BufferWrite([]*spanner.Mutation{
			spanner.Delete("url_mappings", spanner.Key{slug}),
		})
	})
	return err
}

// GetURLStale performs a stale read from Spanner for better performance.
// The staleness bound is configurable.
func (r *spannerRepo) GetURLStale(ctx context.Context, slug string, staleness time.Duration) (string, error) {
	var longURL string
	err := r.client.Single().WithTimestampBound(spanner.MaxStaleness(staleness)).
		Query(ctx, spanner.Statement{
			SQL: "SELECT long_url FROM url_mappings WHERE id = @id",
			Params: map[string]interface{}{
				"id": slug,
			},
		}).Do(func(r *spanner.Row) error {
		return r.Column(0, &longURL)
	})
	return longURL, err
}

func (r *spannerRepo) GetUserHistory(ctx context.Context, userID string) ([]UserHistoryRecord, error) {
	var history []UserHistoryRecord
	iter := r.client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT id, long_url, created_at FROM url_mappings WHERE user_id = @user_id ORDER BY created_at DESC",
		Params: map[string]interface{}{
			"user_id": userID,
		},
	})
	err := iter.Do(func(row *spanner.Row) error {
		var rec UserHistoryRecord
		if err := row.ToStruct(&rec); err != nil {
			return err
		}
		history = append(history, rec)
		return nil
	})
	return history, err
}
