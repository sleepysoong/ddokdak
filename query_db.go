package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
)

func main() {
	db, err := sql.Open("sqlite3", os.Args[1])
	if err != nil { log.Fatal(err) }
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type='table'")
	if err != nil { log.Fatal(err) }
	for rows.Next() {
		var name, sql string
		rows.Scan(&name, &sql)
		fmt.Printf("Table: %s\nSchema: %s\n", name, sql)
	}
}
