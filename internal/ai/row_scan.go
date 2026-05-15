package ai

import (
	"database/sql"
)

func scanSQLRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		holders := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range holders {
			ptrs[i] = &holders[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(colNames))
		for i, name := range colNames {
			v := holders[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[name] = v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
