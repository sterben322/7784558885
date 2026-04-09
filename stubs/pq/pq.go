package pq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
)

type StringArray []string

func Array(a any) any { return a }

func init() {
	sql.Register("postgres", stubDriver{})
}

type stubDriver struct{}

func (stubDriver) Open(name string) (driver.Conn, error) {
	return stubConn{}, nil
}

type stubConn struct{}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("pq stub: statements are not supported")
}

func (stubConn) Close() error { return nil }

func (stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("pq stub: transactions are not supported")
}

func (stubConn) Ping(context.Context) error {
	return errors.New("pq stub: postgres driver is unavailable in this build")
}
