package sinks

// testDB implements a minimal in-memory database/sql driver used by DB sink
// tests. It supports the small subset of SQL that DatabaseSink produces:
//
//   - CREATE TABLE / CREATE INDEX  — ignored (always succeeds)
//   - INSERT INTO <table> (cols) VALUES (?, ?, …)  — stores rows
//   - SELECT COUNT(*) FROM <table> [WHERE col = 'val']
//   - SELECT col1[, col2] FROM <table> WHERE col = 'val'
//
// The driver registers itself under the name "testmemdb".

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

func init() {
	sql.Register("testmemdb", &memDriver{dbs: map[string]*memStore{}})
}

// ─── driver ──────────────────────────────────────────────────────────────────

type memDriver struct {
	mu  sync.Mutex
	dbs map[string]*memStore
}

func (d *memDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.dbs[name]; !ok {
		d.dbs[name] = &memStore{tables: map[string]*memTable{}}
	}
	return &memConn{store: d.dbs[name]}, nil
}

// ─── store ────────────────────────────────────────────────────────────────────

type memStore struct {
	mu     sync.Mutex
	tables map[string]*memTable
}

type memTable struct {
	cols []string
	rows [][]driver.Value
}

func (s *memStore) table(name string) *memTable {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tables[name]; !ok {
		s.tables[name] = &memTable{}
	}
	return s.tables[name]
}

// ─── conn / stmt ──────────────────────────────────────────────────────────────

type memConn struct{ store *memStore }

func (c *memConn) Prepare(query string) (driver.Stmt, error) {
	return &memStmt{store: c.store, query: query}, nil
}
func (c *memConn) Close() error             { return nil }
func (c *memConn) Begin() (driver.Tx, error) { return &memTx{}, nil }

type memTx struct{}

func (t *memTx) Commit() error   { return nil }
func (t *memTx) Rollback() error { return nil }

type memStmt struct {
	store *memStore
	query string
}

func (s *memStmt) Close() error    { return nil }
func (s *memStmt) NumInput() int   { return -1 } // variadic

func (s *memStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	upper := strings.ToUpper(q)

	switch {
	case strings.HasPrefix(upper, "CREATE"), strings.HasPrefix(upper, "DROP"):
		return driver.RowsAffected(0), nil

	case strings.HasPrefix(upper, "INSERT INTO"):
		return s.execInsert(q, args)

	default:
		return driver.RowsAffected(0), nil
	}
}

var reInsert = regexp.MustCompile(`(?is)INSERT\s+INTO\s+(\w+)\s*\(([^)]+)\)\s*VALUES\s*(.+)`)

func (s *memStmt) execInsert(q string, args []driver.Value) (driver.Result, error) {
	m := reInsert.FindStringSubmatch(q)
	if m == nil {
		return nil, fmt.Errorf("testmemdb: cannot parse INSERT: %q", q)
	}
	tableName := strings.ToLower(m[1])
	colNames := splitTrim(m[2])
	nCols := len(colNames)

	tbl := s.store.table(tableName)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	tbl.cols = colNames // idempotent – same every time for a given table

	// Each row is nCols consecutive args.
	for start := 0; start+nCols <= len(args); start += nCols {
		row := make([]driver.Value, nCols)
		copy(row, args[start:start+nCols])
		tbl.rows = append(tbl.rows, row)
	}
	return driver.RowsAffected(int64(len(args) / nCols)), nil
}

func (s *memStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	upper := strings.ToUpper(q)

	if strings.Contains(upper, "COUNT(*)") {
		return s.queryCount(q)
	}
	return s.querySelect(q)
}

// ─── SELECT COUNT(*) FROM table [WHERE col = 'val'] ──────────────────────────

var reCountFrom = regexp.MustCompile(`(?is)SELECT\s+COUNT\(\*\)\s+FROM\s+(\w+)(?:\s+WHERE\s+(\w+)\s*=\s*'([^']*)')?`)

func (s *memStmt) queryCount(q string) (driver.Rows, error) {
	m := reCountFrom.FindStringSubmatch(q)
	if m == nil {
		return nil, fmt.Errorf("testmemdb: cannot parse COUNT query: %q", q)
	}
	tableName := strings.ToLower(m[1])
	filterCol, filterVal := m[2], m[3]

	tbl := s.store.table(tableName)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	count := 0
	for _, row := range tbl.rows {
		if filterCol == "" || matchFilter(tbl.cols, row, filterCol, filterVal) {
			count++
		}
	}
	return &scalarRows{value: int64(count)}, nil
}

// ─── SELECT col1[, col2] FROM table WHERE col = 'val' ───────────────────────

var reSelect = regexp.MustCompile(`(?is)SELECT\s+(.+?)\s+FROM\s+(\w+)(?:\s+WHERE\s+(\w+)\s*=\s*'([^']*)')?`)

func (s *memStmt) querySelect(q string) (driver.Rows, error) {
	m := reSelect.FindStringSubmatch(q)
	if m == nil {
		return nil, fmt.Errorf("testmemdb: cannot parse SELECT: %q", q)
	}
	selectCols := splitTrim(m[1])
	tableName := strings.ToLower(m[2])
	filterCol, filterVal := m[3], m[4]

	tbl := s.store.table(tableName)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	var matched [][]driver.Value
	for _, row := range tbl.rows {
		if filterCol == "" || matchFilter(tbl.cols, row, filterCol, filterVal) {
			projected := projectRow(tbl.cols, row, selectCols)
			matched = append(matched, projected)
		}
	}
	return &tableRows{cols: selectCols, rows: matched}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func matchFilter(cols []string, row []driver.Value, filterCol, filterVal string) bool {
	for i, c := range cols {
		if strings.EqualFold(c, filterCol) {
			return fmt.Sprintf("%v", row[i]) == filterVal
		}
	}
	return false
}

func projectRow(srcCols []string, row []driver.Value, wantCols []string) []driver.Value {
	out := make([]driver.Value, len(wantCols))
	for i, wc := range wantCols {
		for j, sc := range srcCols {
			if strings.EqualFold(sc, wc) {
				out[i] = row[j]
				break
			}
		}
	}
	return out
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

// ─── Rows implementations ─────────────────────────────────────────────────────

// scalarRows returns a single integer value (for COUNT).
type scalarRows struct {
	value int64
	done  bool
}

func (r *scalarRows) Columns() []string { return []string{"count"} }
func (r *scalarRows) Close() error      { return nil }
func (r *scalarRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.value
	return nil
}

// tableRows returns multiple rows of projected columns.
type tableRows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *tableRows) Columns() []string { return r.cols }
func (r *tableRows) Close() error      { return nil }
func (r *tableRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.idx]
	r.idx++
	for i, v := range row {
		dest[i] = v
	}
	return nil
}

// ─── scanValue helpers (for test assertions) ──────────────────────────────────

// scanInt scans a driver.Value (int64 or string) into an int.
func scanInt(v interface{}) (int, error) {
	switch x := v.(type) {
	case int64:
		return int(x), nil
	case string:
		return strconv.Atoi(x)
	default:
		return 0, fmt.Errorf("scanInt: unexpected type %T", v)
	}
}
