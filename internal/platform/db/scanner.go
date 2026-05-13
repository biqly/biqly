package db

// Scanner matches *sql.Row / *sql.Rows scan targets used by repository helpers.
type Scanner interface {
	Scan(dest ...any) error
}
