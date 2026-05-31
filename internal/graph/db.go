// Package graph provides SQLite-backed storage and traversal for the code knowledge graph.
package graph

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/Qxy0happy/codegraph-go/internal/types"
)

// DB wraps a SQLite connection for graph queries.
type DB struct {
	db *sql.DB
}

// Open opens an existing CodeGraph database at the given path.
func Open(dbPath string) (*DB, error) {
	// URI: enable WAL mode, FK enforcement, and FTS5 support
	connStr := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// sql.Open doesn't actually connect until a query is made; verify now
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{db: db}, nil
}

// OpenRW opens (or creates) a CodeGraph database for read-write access.
// Initializes the schema if the database is new.
func OpenRW(dbPath string) (*DB, error) {
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on&_cache_size=-64000", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open rw db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping rw db: %w", err)
	}

	d := &DB{db: db}

	// Initialize schema
	if err := d.initSchema(); err != nil {
		d.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return d, nil
}

// initSchema creates the CodeGraph tables if they don't exist.
func (d *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL,
		name TEXT NOT NULL,
		qualified_name TEXT NOT NULL,
		file_path TEXT NOT NULL,
		language TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		start_column INTEGER NOT NULL,
		end_column INTEGER NOT NULL,
		docstring TEXT,
		signature TEXT,
		visibility TEXT,
		is_exported INTEGER DEFAULT 0,
		is_async INTEGER DEFAULT 0,
		is_static INTEGER DEFAULT 0,
		is_abstract INTEGER DEFAULT 0,
		decorators TEXT,
		type_parameters TEXT,
		updated_at INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		kind TEXT NOT NULL,
		metadata TEXT,
		line INTEGER,
		col INTEGER,
		provenance TEXT DEFAULT NULL,
		FOREIGN KEY (source) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (target) REFERENCES nodes(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS files (
		path TEXT PRIMARY KEY,
		content_hash TEXT NOT NULL DEFAULT '',
		language TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		modified_at INTEGER NOT NULL DEFAULT 0,
		indexed_at INTEGER NOT NULL DEFAULT 0,
		node_count INTEGER DEFAULT 0,
		errors TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(kind);
	CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
	CREATE INDEX IF NOT EXISTS idx_nodes_file_path ON nodes(file_path);
	CREATE INDEX IF NOT EXISTS idx_nodes_lower_name ON nodes(lower(name));
	CREATE INDEX IF NOT EXISTS idx_edges_source_kind ON edges(source, kind);
	CREATE INDEX IF NOT EXISTS idx_edges_target_kind ON edges(target, kind);
	CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);
	CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
		id, name, qualified_name, docstring, signature,
		content='nodes', content_rowid='rowid'
	);
	CREATE TRIGGER IF NOT EXISTS nodes_ai AFTER INSERT ON nodes BEGIN
		INSERT INTO nodes_fts(rowid, id, name, qualified_name, docstring, signature)
		VALUES (NEW.rowid, NEW.id, NEW.name, NEW.qualified_name, NEW.docstring, NEW.signature);
	END;
	CREATE TRIGGER IF NOT EXISTS nodes_ad AFTER DELETE ON nodes BEGIN
		INSERT INTO nodes_fts(nodes_fts, rowid, id, name, qualified_name, docstring, signature)
		VALUES ('delete', OLD.rowid, OLD.id, OLD.name, OLD.qualified_name, OLD.docstring, OLD.signature);
	END;
	CREATE TRIGGER IF NOT EXISTS nodes_au AFTER UPDATE ON nodes BEGIN
		INSERT INTO nodes_fts(nodes_fts, rowid, id, name, qualified_name, docstring, signature)
		VALUES ('delete', OLD.rowid, OLD.id, OLD.name, OLD.qualified_name, OLD.docstring, OLD.signature);
		INSERT INTO nodes_fts(rowid, id, name, qualified_name, docstring, signature)
		VALUES (NEW.rowid, NEW.id, NEW.name, NEW.qualified_name, NEW.docstring, NEW.signature);
	END;
	`
	_, err := d.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// InsertNode adds a node to the database.
func (d *DB) InsertNode(n types.Node) error {
	exported := 0
	if n.IsExported {
		exported = 1
	}
	_, err := d.db.Exec(`INSERT OR REPLACE INTO nodes
		(id, kind, name, qualified_name, file_path, language,
		 start_line, end_line, start_column, end_column,
		 docstring, signature, visibility,
		 is_exported, is_async, is_static, is_abstract,
		 updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, string(n.Kind), n.Name, n.QualifiedName, n.FilePath, string(n.Language),
		n.StartLine, n.EndLine, n.StartColumn, n.EndColumn,
		nil, n.Signature, n.Visibility,
		exported, 0, 0, 0,
		n.UpdatedAt,
	)
	return err
}

// InsertEdge adds an edge to the database.
func (d *DB) InsertEdge(e types.Edge) error {
	_, err := d.db.Exec(`INSERT INTO edges
		(source, target, kind, metadata, line, col, provenance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Source, e.Target, string(e.Kind),
		e.Metadata, e.Line, e.Col, e.Provenance,
	)
	return err
}

// UpsertFile inserts or updates a file record.
func (d *DB) UpsertFile(f types.File) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO files
		(path, language, node_count, indexed_at)
		VALUES (?, ?, ?, ?)`,
		f.Path, f.Language, f.NodeCount, f.IndexedAt,
	)
	return err
}

// GetNodeByID retrieves a single node by its ID.
func (d *DB) GetNodeByID(id string) (*types.Node, error) {
	row := d.db.QueryRow(`
		SELECT id, kind, name, qualified_name, file_path, language,
		       start_line, end_line, start_column, end_column,
		       docstring, signature, visibility,
		       is_exported, is_async, is_static, is_abstract,
		       decorators, type_parameters, updated_at
		FROM nodes WHERE id = ?`, id)
	return scanNode(row)
}

// GetNodesByFile returns all nodes in a file, ordered by line.
func (d *DB) GetNodesByFile(filePath string) ([]types.Node, error) {
	rows, err := d.db.Query(`
		SELECT id, kind, name, qualified_name, file_path, language,
		       start_line, end_line, start_column, end_column,
		       docstring, signature, visibility,
		       is_exported, is_async, is_static, is_abstract,
		       decorators, type_parameters, updated_at
		FROM nodes WHERE file_path = ? ORDER BY start_line`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetNodesByKind returns all nodes of a given kind.
func (d *DB) GetNodesByKind(kind types.NodeKind) ([]types.Node, error) {
	rows, err := d.db.Query(`
		SELECT id, kind, name, qualified_name, file_path, language,
		       start_line, end_line, start_column, end_column,
		       docstring, signature, visibility,
		       is_exported, is_async, is_static, is_abstract,
		       decorators, type_parameters, updated_at
		FROM nodes WHERE kind = ? ORDER BY name`, string(kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// SearchNodes performs a full-text search on node names and docstrings.
func (d *DB) SearchNodes(query string, limit int) ([]types.Node, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// Use FTS5 for ranked search
	rows, err := d.db.Query(`
		SELECT n.id, n.kind, n.name, n.qualified_name, n.file_path, n.language,
		       n.start_line, n.end_line, n.start_column, n.end_column,
		       n.docstring, n.signature, n.visibility,
		       n.is_exported, n.is_async, n.is_static, n.is_abstract,
		       n.decorators, n.type_parameters, n.updated_at
		FROM nodes_fts f
		JOIN nodes n ON n.id = f.id
		WHERE nodes_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// GetEdgesBySource returns all edges originating from a node.
func (d *DB) GetEdgesBySource(sourceID string, kind ...types.EdgeKind) ([]types.Edge, error) {
	q := `SELECT id, source, target, kind, metadata, line, col, provenance
		  FROM edges WHERE source = ?`
	args := []interface{}{sourceID}
	if len(kind) > 0 {
		placeholders := make([]string, len(kind))
		for i, k := range kind {
			placeholders[i] = "?"
			args = append(args, string(k))
		}
		q += " AND kind IN (" + strings.Join(placeholders, ",") + ")"
	}
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// GetEdgesByTarget returns all edges pointing to a node.
func (d *DB) GetEdgesByTarget(targetID string, kind ...types.EdgeKind) ([]types.Edge, error) {
	q := `SELECT id, source, target, kind, metadata, line, col, provenance
		  FROM edges WHERE target = ?`
	args := []interface{}{targetID}
	if len(kind) > 0 {
		placeholders := make([]string, len(kind))
		for i, k := range kind {
			placeholders[i] = "?"
			args = append(args, string(k))
		}
		q += " AND kind IN (" + strings.Join(placeholders, ",") + ")"
	}
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// GetCallers returns nodes that call the given node, with their calling edge.
func (d *DB) GetCallers(nodeID string) ([]types.Edge, error) {
	return d.GetEdgesByTarget(nodeID, types.EdgeCalls)
}

// GetCallees returns nodes called by the given node, with their calling edge.
func (d *DB) GetCallees(nodeID string) ([]types.Edge, error) {
	return d.GetEdgesBySource(nodeID, types.EdgeCalls)
}

// GetNodeCount returns the total number of indexed nodes.
func (d *DB) GetNodeCount() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count)
	return count, err
}

// GetEdgeCount returns the total number of edges.
func (d *DB) GetEdgeCount() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM edges").Scan(&count)
	return count, err
}

// ---------------------------------------------------------------------------
// Scan helpers
// ---------------------------------------------------------------------------

func scanNode(s scanner) (*types.Node, error) {
	var n types.Node
	var docstring, signature, visibility, decorators, typeParams sql.NullString
	var exported, async, stat, abstract int
	err := s.Scan(
		&n.ID, &n.Kind, &n.Name, &n.QualifiedName, &n.FilePath, &n.Language,
		&n.StartLine, &n.EndLine, &n.StartColumn, &n.EndColumn,
		&docstring, &signature, &visibility,
		&exported, &async, &stat, &abstract,
		&decorators, &typeParams, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if docstring.Valid {
		n.Docstring = &docstring.String
	}
	if signature.Valid {
		n.Signature = &signature.String
	}
	if visibility.Valid {
		n.Visibility = &visibility.String
	}
	n.IsExported = exported != 0
	n.IsAsync = async != 0
	n.IsStatic = stat != 0
	n.IsAbstract = abstract != 0
	// JSON arrays can be lazily parsed; for now store as raw strings
	if decorators.Valid {
		_ = decorators.String
	}
	if typeParams.Valid {
		_ = typeParams.String
	}
	return &n, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanNodes(rows *sql.Rows) ([]types.Node, error) {
	var nodes []types.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *n)
	}
	return nodes, rows.Err()
}

func scanEdge(s scanner) (*types.Edge, error) {
	var e types.Edge
	var metadata, provenance sql.NullString
	var line, col sql.NullInt64
	err := s.Scan(&e.ID, &e.Source, &e.Target, &e.Kind, &metadata, &line, &col, &provenance)
	if err != nil {
		return nil, err
	}
	if metadata.Valid {
		e.Metadata = &metadata.String
	}
	if line.Valid {
		v := int(line.Int64)
		e.Line = &v
	}
	if col.Valid {
		v := int(col.Int64)
		e.Col = &v
	}
	if provenance.Valid {
		e.Provenance = &provenance.String
	}
	return &e, nil
}

func scanEdges(rows *sql.Rows) ([]types.Edge, error) {
	var edges []types.Edge
	for rows.Next() {
		e, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, *e)
	}
	return edges, rows.Err()
}
