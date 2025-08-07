package tran

import "go_project/ms_project/project_user/internal/database"

type Transaction interface {
	Action(func(conn database.DbConn) error) error
}
