// Package store 负责所有实体的 SQLite 持久化：建表迁移与 CRUD。
// 使用纯 Go 驱动 modernc.org/sqlite（CGO 无关），离线可构建。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"task240-fedlineage/internal/model"
)

// Store 封装数据库连接与读写操作。
type Store struct {
	db *sql.DB
}

// Open 打开（不存在则创建）SQLite 数据库并建立表结构。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，简化并发。
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set wal: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS global_models (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL DEFAULT '',
			param_digest TEXT NOT NULL,
			dimension INTEGER NOT NULL,
			round_id TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS aggregate_rounds (
			id TEXT PRIMARY KEY,
			parent_round TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL,
			expected_dim INTEGER NOT NULL,
			sealed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			closed_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS client_updates (
			id TEXT PRIMARY KEY,
			round_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			parent_model TEXT NOT NULL DEFAULT '',
			param_digest TEXT NOT NULL,
			dimension INTEGER NOT NULL,
			state TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS lineage_edges (
			child TEXT NOT NULL,
			parent TEXT NOT NULL,
			PRIMARY KEY (child, parent)
		);`,
		`CREATE TABLE IF NOT EXISTS round_snapshots (
			id TEXT PRIMARY KEY,
			round_id TEXT NOT NULL,
			state TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS verifications (
			update_id TEXT NOT NULL,
			round_id TEXT NOT NULL,
			verdict TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			verified_at TEXT NOT NULL,
			PRIMARY KEY (update_id, round_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_updates_round ON client_updates(round_id);`,
		`CREATE INDEX IF NOT EXISTS idx_models_round ON global_models(round_id);`,
		`CREATE INDEX IF NOT EXISTS idx_edges_parent ON lineage_edges(parent);`,
	}
	for _, st := range stmts {
		if _, err := db.Exec(st); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- GlobalModel ----

// PutModel 写入或更新模型节点。
func (s *Store) PutModel(m *model.GlobalModel) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO global_models (id, parent_id, param_digest, dimension, round_id, state, created_at)
		 VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET parent_id=excluded.parent_id, param_digest=excluded.param_digest,
		   dimension=excluded.dimension, round_id=excluded.round_id, state=excluded.state;`,
		m.ID, m.ParentID, m.ParamDigest, m.Dimension, m.RoundID, m.State, nowStr())
	if err != nil {
		return fmt.Errorf("put model: %w", err)
	}
	return nil
}

// GetModel 读取模型节点。
func (s *Store) GetModel(id string) (*model.GlobalModel, error) {
	row := s.db.QueryRow(
		`SELECT id, parent_id, param_digest, dimension, round_id, state, created_at FROM global_models WHERE id=?`, id)
	m := &model.GlobalModel{}
	var ca string
	if err := row.Scan(&m.ID, &m.ParentID, &m.ParamDigest, &m.Dimension, &m.RoundID, &m.State, &ca); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get model: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return m, nil
}

// ListModels 列出全部模型节点。
func (s *Store) ListModels() ([]*model.GlobalModel, error) {
	rows, err := s.db.Query(`SELECT id, parent_id, param_digest, dimension, round_id, state, created_at FROM global_models ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()
	out := []*model.GlobalModel{}
	for rows.Next() {
		m := &model.GlobalModel{}
		var ca string
		if err := rows.Scan(&m.ID, &m.ParentID, &m.ParamDigest, &m.Dimension, &m.RoundID, &m.State, &ca); err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, m)
	}
	return out, nil
}

// ---- AggregateRound ----

// PutRound 写入或更新聚合轮次。
func (s *Store) PutRound(r *model.AggregateRound) error {
	_, err := s.db.Exec(
		`INSERT INTO aggregate_rounds (id, parent_round, state, expected_dim, sealed_at, created_at, closed_at)
		 VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET parent_round=excluded.parent_round, state=excluded.state,
		   expected_dim=excluded.expected_dim, sealed_at=excluded.sealed_at, closed_at=excluded.closed_at;`,
		r.ID, r.ParentRound, r.State, r.ExpectedDim, r.SealedAt.Format(time.RFC3339), nowStr(), r.ClosedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("put round: %w", err)
	}
	return nil
}

// GetRound 读取聚合轮次。
func (s *Store) GetRound(id string) (*model.AggregateRound, error) {
	row := s.db.QueryRow(
		`SELECT id, parent_round, state, expected_dim, sealed_at, created_at, closed_at FROM aggregate_rounds WHERE id=?`, id)
	r := &model.AggregateRound{}
	var sa, ca, cla string
	if err := row.Scan(&r.ID, &r.ParentRound, &r.State, &r.ExpectedDim, &sa, &ca, &cla); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get round: %w", err)
	}
	r.SealedAt, _ = time.Parse(time.RFC3339, sa)
	r.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	r.ClosedAt, _ = time.Parse(time.RFC3339, cla)
	return r, nil
}

// ListRounds 列出全部轮次。
func (s *Store) ListRounds() ([]*model.AggregateRound, error) {
	rows, err := s.db.Query(`SELECT id, parent_round, state, expected_dim, sealed_at, created_at, closed_at FROM aggregate_rounds ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list rounds: %w", err)
	}
	defer rows.Close()
	out := []*model.AggregateRound{}
	for rows.Next() {
		r := &model.AggregateRound{}
		var sa, ca, cla string
		if err := rows.Scan(&r.ID, &r.ParentRound, &r.State, &r.ExpectedDim, &sa, &ca, &cla); err != nil {
			return nil, fmt.Errorf("scan round: %w", err)
		}
		r.SealedAt, _ = time.Parse(time.RFC3339, sa)
		r.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		r.ClosedAt, _ = time.Parse(time.RFC3339, cla)
		out = append(out, r)
	}
	return out, nil
}

// ---- ClientUpdate ----

// PutUpdate 写入或更新客户端更新（id 为幂等键）。
func (s *Store) PutUpdate(u *model.ClientUpdate) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO client_updates (id, round_id, client_id, parent_model, param_digest, dimension, state, reason, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET round_id=excluded.round_id, client_id=excluded.client_id,
		   parent_model=excluded.parent_model, param_digest=excluded.param_digest, dimension=excluded.dimension,
		   state=excluded.state, reason=excluded.reason;`,
		u.ID, u.RoundID, u.ClientID, u.ParentModel, u.ParamDigest, u.Dimension, u.State, u.Reason, nowStr())
	if err != nil {
		return fmt.Errorf("put update: %w", err)
	}
	return nil
}

// GetUpdate 读取客户端更新。
func (s *Store) GetUpdate(id string) (*model.ClientUpdate, error) {
	row := s.db.QueryRow(
		`SELECT id, round_id, client_id, parent_model, param_digest, dimension, state, reason, created_at FROM client_updates WHERE id=?`, id)
	u := &model.ClientUpdate{}
	var ca string
	if err := row.Scan(&u.ID, &u.RoundID, &u.ClientID, &u.ParentModel, &u.ParamDigest, &u.Dimension, &u.State, &u.Reason, &ca); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get update: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return u, nil
}

// ListUpdatesByRound 列出某轮次全部更新（含隔离）。
func (s *Store) ListUpdatesByRound(roundID string) ([]*model.ClientUpdate, error) {
	rows, err := s.db.Query(
		`SELECT id, round_id, client_id, parent_model, param_digest, dimension, state, reason, created_at FROM client_updates WHERE round_id=? ORDER BY created_at`, roundID)
	if err != nil {
		return nil, fmt.Errorf("list updates: %w", err)
	}
	defer rows.Close()
	out := []*model.ClientUpdate{}
	for rows.Next() {
		u := &model.ClientUpdate{}
		var ca string
		if err := rows.Scan(&u.ID, &u.RoundID, &u.ClientID, &u.ParentModel, &u.ParamDigest, &u.Dimension, &u.State, &u.Reason, &ca); err != nil {
			return nil, fmt.Errorf("scan update: %w", err)
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, u)
	}
	return out, nil
}

// CountUpdatesByRoundState 统计某轮次某状态的更新数。
func (s *Store) CountUpdatesByRoundState(roundID, state string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM client_updates WHERE round_id=? AND state=?`, roundID, state).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count updates: %w", err)
	}
	return n, nil
}

// ---- LineageEdge ----

// PutEdge 写入谱系边（幂等）。
func (s *Store) PutEdge(e model.LineageEdge) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO lineage_edges (child, parent) VALUES (?,?)`, e.Child, e.Parent)
	if err != nil {
		return fmt.Errorf("put edge: %w", err)
	}
	return nil
}

// ParentsOf 返回某模型节点的所有父节点 ID。
func (s *Store) ParentsOf(child string) ([]string, error) {
	rows, err := s.db.Query(`SELECT parent FROM lineage_edges WHERE child=?`, child)
	if err != nil {
		return nil, fmt.Errorf("parents: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan parent: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

// ChildrenOf 返回某模型节点的所有子节点 ID。
func (s *Store) ChildrenOf(parent string) ([]string, error) {
	rows, err := s.db.Query(`SELECT child FROM lineage_edges WHERE parent=?`, parent)
	if err != nil {
		return nil, fmt.Errorf("children: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan child: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// AllEdges 返回全部谱系边。
func (s *Store) AllEdges() ([]model.LineageEdge, error) {
	rows, err := s.db.Query(`SELECT child, parent FROM lineage_edges`)
	if err != nil {
		return nil, fmt.Errorf("all edges: %w", err)
	}
	defer rows.Close()
	out := []model.LineageEdge{}
	for rows.Next() {
		var c, p string
		if err := rows.Scan(&c, p); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		out = append(out, model.LineageEdge{Child: c, Parent: p})
	}
	return out, nil
}

// ---- RoundSnapshot ----

// PutSnapshot 写入或更新快照。
func (s *Store) PutSnapshot(snap *model.RoundSnapshot) error {
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO round_snapshots (id, round_id, state, summary, created_at)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET round_id=excluded.round_id, state=excluded.state, summary=excluded.summary;`,
		snap.ID, snap.RoundID, snap.State, snap.Summary, nowStr())
	if err != nil {
		return fmt.Errorf("put snapshot: %w", err)
	}
	return nil
}

// GetSnapshot 读取快照。
func (s *Store) GetSnapshot(id string) (*model.RoundSnapshot, error) {
	row := s.db.QueryRow(`SELECT id, round_id, state, summary, created_at FROM round_snapshots WHERE id=?`, id)
	snap := &model.RoundSnapshot{}
	var ca string
	if err := row.Scan(&snap.ID, &snap.RoundID, &snap.State, &snap.Summary, &ca); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	snap.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return snap, nil
}

// ListSnapshotsByRound 列出某轮次全部快照。
func (s *Store) ListSnapshotsByRound(roundID string) ([]*model.RoundSnapshot, error) {
	rows, err := s.db.Query(`SELECT id, round_id, state, summary, created_at FROM round_snapshots WHERE round_id=? ORDER BY created_at`, roundID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	out := []*model.RoundSnapshot{}
	for rows.Next() {
		snap := &model.RoundSnapshot{}
		var ca string
		if err := rows.Scan(&snap.ID, &snap.RoundID, &snap.State, &snap.Summary, &ca); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snap.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, snap)
	}
	return out, nil
}

// GetPublishedSnapshot 返回某轮次已发布的快照（若有）。
func (s *Store) GetPublishedSnapshot(roundID string) (*model.RoundSnapshot, error) {
	row := s.db.QueryRow(`SELECT id, round_id, state, summary, created_at FROM round_snapshots WHERE round_id=? AND state=? ORDER BY created_at DESC LIMIT 1`, roundID, model.SnapshotStatePublish)
	snap := &model.RoundSnapshot{}
	var ca string
	if err := row.Scan(&snap.ID, &snap.RoundID, &snap.State, &snap.Summary, &ca); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("get published snapshot: %w", err)
	}
	snap.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return snap, nil
}

// ---- Verification ----

// PutVerification 写入校验结果（update_id+round_id 唯一）。
func (s *Store) PutVerification(v model.UpdateVerification) error {
	if v.VerifiedAt.IsZero() {
		v.VerifiedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO verifications (update_id, round_id, verdict, reason, verified_at)
		 VALUES (?,?,?,?,?)
		 ON CONFLICT(update_id, round_id) DO UPDATE SET verdict=excluded.verdict, reason=excluded.reason, verified_at=excluded.verified_at;`,
		v.UpdateID, v.RoundID, v.Verdict, v.Reason, v.VerifiedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("put verification: %w", err)
	}
	return nil
}

// ListVerificationsByRound 列出某轮次全部校验结果。
func (s *Store) ListVerificationsByRound(roundID string) ([]model.UpdateVerification, error) {
	rows, err := s.db.Query(`SELECT update_id, round_id, verdict, reason, verified_at FROM verifications WHERE round_id=? ORDER BY verified_at`, roundID)
	if err != nil {
		return nil, fmt.Errorf("list verifications: %w", err)
	}
	defer rows.Close()
	out := []model.UpdateVerification{}
	for rows.Next() {
		v := model.UpdateVerification{}
		var va string
		if err := rows.Scan(&v.UpdateID, &v.RoundID, &v.Verdict, &v.Reason, &va); err != nil {
			return nil, fmt.Errorf("scan verification: %w", err)
		}
		v.VerifiedAt, _ = time.Parse(time.RFC3339, va)
		out = append(out, v)
	}
	return out, nil
}
