package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"time"
)

type WebRevocation struct {
	webPrivate
	ID, RefreshToken string
	Revision         int64
	Attempts         int
}
type WebRevocationStatus struct{ Pending, Running, Completed, Attention int64 }

func (b *BoundWebSessionStore) RevocationStatus(ctx context.Context) (WebRevocationStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var result WebRevocationStatus
	err := b.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE state='pending'),count(*) FILTER(WHERE state='running'),count(*) FILTER(WHERE state='done'),count(*) FILTER(WHERE state='attention') FROM auth.web_revocations WHERE client_hash=$1`, b.clientHash()).Scan(&result.Pending, &result.Running, &result.Completed, &result.Attention)
	if err != nil {
		return WebRevocationStatus{}, ErrStore
	}
	return result, nil
}

type BoundWebSessionStore struct {
	*WebSessionStore
	issuer, client string
}

func NewBoundWebSessionStore(s *WebSessionStore, issuer, client string) (*BoundWebSessionStore, error) {
	if s == nil || issuer == "" || len(issuer) > 2048 || client == "" || len(client) > 256 {
		return nil, ErrStore
	}
	return &BoundWebSessionStore{s, issuer, client}, nil
}
func (b *BoundWebSessionStore) clientHash() []byte {
	v, _ := json.Marshal([]string{"govai-web-revocation-client-v1", b.issuer, b.client})
	h := sha256.Sum256(v)
	return h[:]
}
func (b *BoundWebSessionStore) sealRevocation(raw string) (string, []byte, error) {
	if raw == "" || len(raw) > 16<<10 {
		return "", nil, ErrStore
	}
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", nil, ErrStore
	}
	job := base64.RawURLEncoding.EncodeToString(id[:])
	plain, _ := json.Marshal(map[string]string{"issuer": b.issuer, "client": b.client, "refresh": raw})
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, ErrStore
	}
	return job, b.aead.Seal(nonce, nonce, plain, []byte("govai-web-revocation-v1\x00"+job)), nil
}
func (b *BoundWebSessionStore) openRevocation(job string, sealed []byte) (string, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n || len(sealed) > 20000 {
		return "", ErrStore
	}
	plain, err := b.aead.Open(nil, sealed[:n], sealed[n:], []byte("govai-web-revocation-v1\x00"+job))
	if err != nil {
		return "", ErrStore
	}
	var p map[string]string
	if json.Unmarshal(plain, &p) != nil || p["issuer"] != b.issuer || p["client"] != b.client || p["refresh"] == "" || len(p["refresh"]) > 16<<10 {
		return "", ErrStore
	}
	return p["refresh"], nil
}

// TerminateQueued atomically invalidates local custody and saves the last known
// provider refresh credential. Commit ambiguity returns no success; the database
// never commits local termination without its recovery record.
func (b *BoundWebSessionStore) TerminateQueued(ctx context.Context, raw string) (string, error) {
	hash, err := webHash(raw)
	if err != nil {
		return "", ErrWebSessionMissing
	}
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return "", ErrStore
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var encrypted []byte
	if err = tx.QueryRow(ctx, "SELECT ciphertext FROM auth.web_sessions WHERE session_hash=$1 AND state IN ('active','refreshing') FOR UPDATE", hash).Scan(&encrypted); err != nil {
		return "", webReadError(err)
	}
	c, err := b.open(encrypted, hash)
	if err != nil || c.Issuer != b.issuer {
		return "", ErrStore
	}
	id, sealed, err := b.sealRevocation(c.RefreshToken)
	if err != nil {
		return "", ErrStore
	}
	if _, err = tx.Exec(ctx, "INSERT INTO auth.web_revocations(job_id,client_hash,ciphertext) VALUES($1,$2,$3)", id, b.clientHash(), sealed); err != nil {
		return "", ErrStore
	}
	if _, err = tx.Exec(ctx, "UPDATE auth.web_sessions SET state='revoked',revision=revision+1 WHERE session_hash=$1", hash); err != nil {
		return "", ErrStore
	}
	if tx.Commit(ctx) != nil {
		return "", ErrStore
	}
	return id, nil
}

// QueueRevocation retains a known newly rotated credential when a later local
// commit fails. It never attempts another refresh-token exchange.
func (b *BoundWebSessionStore) QueueRevocation(ctx context.Context, issuer, raw string) (string, error) {
	if issuer != b.issuer {
		return "", ErrStore
	}
	id, sealed, err := b.sealRevocation(raw)
	if err != nil {
		return "", ErrStore
	}
	if _, err = b.pool.Exec(ctx, "INSERT INTO auth.web_revocations(job_id,client_hash,ciphertext) VALUES($1,$2,$3)", id, b.clientHash(), sealed); err != nil {
		return "", ErrStore
	}
	return id, nil
}

// ClaimRevocation leases at most one due job for 30 seconds. Revocation itself
// is idempotent, so a lease-expired request may safely be repeated. Eight failed
// attempts stop in attention state, retaining encrypted operator evidence.
func (b *BoundWebSessionStore) ClaimRevocation(ctx context.Context) (WebRevocation, error) {
	if _, err := b.pool.Exec(ctx, "UPDATE auth.web_revocations SET state='attention',revision=revision+1,updated_at=clock_timestamp() WHERE client_hash=$1 AND state='running' AND attempts>=8 AND next_attempt<=clock_timestamp()", b.clientHash()); err != nil {
		return WebRevocation{}, ErrStore
	}
	var job WebRevocation
	var sealed []byte
	err := b.pool.QueryRow(ctx, `UPDATE auth.web_revocations SET state='running',revision=revision+1,attempts=attempts+1,next_attempt=clock_timestamp()+interval '30 seconds',updated_at=clock_timestamp()
 WHERE job_id=(SELECT job_id FROM auth.web_revocations WHERE client_hash=$1 AND state IN ('pending','running') AND next_attempt<=clock_timestamp() AND attempts<8 ORDER BY next_attempt,job_id LIMIT 1 FOR UPDATE SKIP LOCKED)
 RETURNING job_id,ciphertext,revision,attempts`, b.clientHash()).Scan(&job.ID, &sealed, &job.Revision, &job.Attempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return WebRevocation{}, ErrWebSessionMissing
		}
		return WebRevocation{}, ErrStore
	}
	job.RefreshToken, err = b.openRevocation(job.ID, sealed)
	if err != nil {
		return WebRevocation{}, ErrStore
	}
	return job, nil
}
func (b *BoundWebSessionStore) CompleteRevocation(ctx context.Context, id string) error {
	tag, err := b.pool.Exec(ctx, "UPDATE auth.web_revocations SET state='done',ciphertext=''::bytea,revision=revision+1,updated_at=clock_timestamp() WHERE job_id=$1 AND client_hash=$2 AND state<>'done'", id, b.clientHash())
	if err != nil {
		return ErrStore
	}
	_ = tag
	return nil
}

// LoadRevocation reads the exact credential captured by atomic termination.
// A pre-termination browser snapshot may contain an older rotated credential.
func (b *BoundWebSessionStore) LoadRevocation(ctx context.Context, id string) (WebRevocation, error) {
	var job WebRevocation
	var sealed []byte
	err := b.pool.QueryRow(ctx, "SELECT job_id,ciphertext,revision,attempts FROM auth.web_revocations WHERE job_id=$1 AND client_hash=$2 AND state IN ('pending','running')", id, b.clientHash()).Scan(&job.ID, &sealed, &job.Revision, &job.Attempts)
	if err != nil {
		return WebRevocation{}, webReadError(err)
	}
	job.RefreshToken, err = b.openRevocation(job.ID, sealed)
	if err != nil {
		return WebRevocation{}, ErrStore
	}
	return job, nil
}
func (b *BoundWebSessionStore) RetryRevocation(ctx context.Context, job WebRevocation) error {
	delay := time.Duration(1<<min(job.Attempts, 8)) * time.Second
	tag, err := b.pool.Exec(ctx, `UPDATE auth.web_revocations SET state=CASE WHEN attempts>=8 THEN 'attention' ELSE 'pending' END,next_attempt=clock_timestamp()+($4::bigint*interval '1 second'),revision=revision+1,updated_at=clock_timestamp() WHERE job_id=$1 AND client_hash=$2 AND revision=$3 AND state='running'`, job.ID, b.clientHash(), job.Revision, int64(delay.Seconds()))
	if err != nil || tag.RowsAffected() != 1 {
		return ErrStore
	}
	return nil
}
