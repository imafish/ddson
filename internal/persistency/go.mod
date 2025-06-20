module persistency

go 1.24

replace internal/database => ../../internal/database

require internal/database v0.0.0

require github.com/mattn/go-sqlite3 v1.14.28 // indirect
