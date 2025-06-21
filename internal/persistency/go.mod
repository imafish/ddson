module persistency

go 1.24.4

replace internal/database => ../database

require internal/database v0.0.0

require github.com/mattn/go-sqlite3 v1.14.28 // indirect
