package nagayatesting

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/aereal/nagaya"
	_ "github.com/go-sql-driver/mysql"
)

const envTestDBDSN = "TEST_DB_DSN"

var ErrDSNRequired = errors.New(fmt.Sprintf("%s is required", envTestDBDSN))

func NewMySQLNagayaForTesting() (*nagaya.Nagaya[*sql.DB, *sql.Conn], error) {
	dsn := os.Getenv(envTestDBDSN)
	if dsn == "" {
		return nil, ErrDSNRequired
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return nagaya.NewStd(db), nil
}
